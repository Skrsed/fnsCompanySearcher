package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"log/slog"

	"github.com/Skrsed/fnsCompanySearcher/domain"
	"github.com/spf13/viper"
	"github.com/xuri/excelize/v2"

	_ "github.com/mattn/go-sqlite3"
)

const url = "https://api-fns.ru/api/multinfo"

// example secret = "548d6fa5824faed03dfd6575e2151a3455e3aee6"
const source = "source.xlsx"
const result = "result.xlsx"
const defaultSheet = "Лист1"
const needle = "ОГРН"

var needsToexit bool
var secret string

func main() {
	fmt.Println("There is FNS searcher program. Please set the source file in root folder and enter your key...")
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	slog.Info("Reading file...")

	sourceRows, err := readSource(source)
	if err != nil {
		slog.Error("Error while reading sorce file: " + err.Error())
	}

	ogrns, err := getOgrns(sourceRows)

	log.Printf("ogrns from file len is %v", len(ogrns))

	purified := make([]string, 0, len(ogrns))

	for _, val := range ogrns {
		if _, isNumber := strconv.Atoi(val); isNumber == nil && (len(val) == 13 || len(val) == 15) {
			purified = append(purified, val)
		}
	}

	ogrns = purified

	if err != nil {
		slog.Error("Error while searching ogrns in rows: " + err.Error())
	}

	companiesData := make([]domain.Company, len(ogrns))

	cachedData, err := dbData(ogrns)

	log.Printf("cached len is %v", len(cachedData))

	if err != nil {
		slog.Error("Error while reading db cache " + err.Error())
	}

	cachedOgrns, err := dbCachedOgrns()
	if err != nil {
		slog.Error("Error while scanning cached ogrns result " + err.Error())
	}

	for _, cachedCompany := range cachedData {
		companiesData = append(companiesData, cachedCompany)
		cachedOgrns = append(cachedOgrns, cachedCompany.OGRN)
	}

	if !slices.Contains(os.Args, "--no-cache") {
		newOgrns := make([]string, 0, len(ogrns))
		for _, ogrn := range ogrns {
			if !slices.Contains(cachedOgrns, ogrn) {
				newOgrns = append(newOgrns, ogrn)
			}
		}

		ogrns = newOgrns
	}
	bucket := make([]string, len(ogrns))

	copy(bucket, ogrns)

	slog.Info("First five would be:")
	for i := 0; i < 5 && i < len(bucket); i++ {
		fmt.Println(i, " ", bucket[i])
	}

	fmt.Printf("bucket data %v\n, copied from ogrns %v\n", len(bucket), len(ogrns))

	chunks := make([][]string, 0, len(bucket)/100)

	for len(bucket) != 0 {
		size := min(len(bucket), 100)
		chunks = append(chunks, bucket[:size])
		bucket = bucket[size:]
	}

	groups := make([][][]string, len(chunks)/10+1)
	for i, chunk := range chunks {
		groups[i/10] = append(groups[i/10], chunk)
	}

	slog.Info("Fetching api")

	if !slices.Contains(os.Args, "--only-cache") {
		for _, group := range groups {
			wg := new(sync.WaitGroup)
			chunkData := make(chan []domain.Company, 10)
			for _, chunk := range group {
				wg.Add(1)
				go func(wg *sync.WaitGroup, chunkData chan []domain.Company) {
					fmt.Println(len(chunk))
					chunkData <- getApiData(chunk)
					wg.Done()
				}(wg, chunkData)
				time.Sleep(100 * time.Millisecond)
			}
			wg.Wait()

			for i := 0; i < len(group); i++ {
				companiesData = append(companiesData, <-chunkData...)
			}

			if needsToexit {
				break
			}
		}
	}

	merged := mergeData(sourceRows, companiesData)

	writeToFile(merged)

	slog.Info(fmt.Sprintf("Done, chunks - %v, total - %v", len(chunks), len(ogrns)))

	<-c
}

func dbCachedOgrns() ([]string, error) {
	db, err := sql.Open("sqlite3", "file:./db/store.sqlite?cache=shared")

	if err != nil {
		slog.Error("Error while connecting db " + err.Error())
	}

	defer db.Close()

	rows, err := db.Query("SELECT ogrn FROM main.Cached")
	if err != nil {
		return nil, err
	}

	ogrns := []string{}
	for rows.Next() {
		var ogrn string
		err := rows.Scan(&ogrn)
		if err != nil {
			return nil, err
		}

		ogrns = append(ogrns, ogrn)
	}

	return ogrns, nil
}

func findCompanyByOGRN(companies []domain.Company, ogrn string) (domain.Company, error) {
	for _, company := range companies {
		if company.OGRN == ogrn {
			return company, nil
		}
	}

	return domain.Company{}, errors.New("company not found")
}

func mergeData(rows [][]string, companies []domain.Company) [][]string {
	var needleIndex int
	// Get header row
	for _, headerRow := range rows[:1] {
		for i, colCell := range headerRow {
			if colCell == needle {
				needleIndex = i
			}
		}
	}

	mergedSlice := make([][]string, 0, len(rows))

	mergedSlice = append(mergedSlice, append(rows[0], "Контакты", "Финансы", "ИНН", "CEO"))

	for _, row := range rows[1:] {
		company, err := findCompanyByOGRN(companies, row[needleIndex])
		if err != nil {
			mergedSlice = append(mergedSlice, append(row, "Нет данных"))
			continue
		}
		mergedSlice = append(mergedSlice, append(row, company.Contacts, company.Finances, company.INN, company.CEO))

	}

	return mergedSlice
}

func unmarshalResponse(jsonData []byte) domain.Response {
	var response domain.Response
	err := json.Unmarshal(jsonData, &response)
	if err != nil {
		slog.Error("Error unmarshalling JSON: " + err.Error())
		slog.Info(string(jsonData))

		exit()
	}

	return response
}

func apiCall(ogrns []string) (domain.Response, error) {
	if len(ogrns) == 0 {
		return domain.Response{}, errors.New("empty ogrns chunk")
	}

	fmt.Println("api was called")

	client := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		slog.Error("error while constructing request " + err.Error())
		os.Exit(1)
	}

	q := req.URL.Query()
	q.Add("key", getSecret())
	q.Add("req", strings.Join(ogrns, ","))
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		slog.Error("error while fetching data " + err.Error())
	}

	defer resp.Body.Close()
	contents, _ := io.ReadAll(resp.Body)

	res := unmarshalResponse(contents)

	return res, nil
}

func getSecret() string {
	if secret == "" {
		viper.SetConfigFile(".env")
		viper.ReadInConfig()

		secret = viper.GetString("API_KEY")
	}

	return secret
}

func save(companies []domain.Company) {
	// optimize connections with mutex
	db, err := sql.Open("sqlite3", "file:./db/store.sqlite?cache=shared")

	if err != nil {
		slog.Error("Error while connecting db " + err.Error())
	}

	defer db.Close()

	// Insert data into the database
	for _, company := range companies {
		insertCompany(db, company)
	}
}

func setCachedOgrn(ogrn string) {
	// optimize connections with mutex
	db, err := sql.Open("sqlite3", "file:./db/store.sqlite?cache=shared")

	if err != nil {
		slog.Error("Error while connecting db " + err.Error())
	}

	defer db.Close()

	// optimize with string builder
	_, err = db.Exec(
		"INSERT OR REPLACE INTO main.Cached (ogrn) VALUES (?)",
		ogrn,
	)

	if err != nil {
		log.Fatal(err)
	}
}

func insertCompany(db *sql.DB, company domain.Company) {
	_, err := db.Exec(
		"INSERT OR REPLACE INTO main.Companies (ogrn, finances, contacts, inn, ceo) VALUES (?, ?, ?, ?, ?)",
		company.OGRN,
		company.Finances,
		company.Contacts,
		company.INN,
		company.CEO,
	)

	if err != nil {
		log.Fatal(err)
	}
}

func convertToCompany(item domain.Item) domain.Company {
	var company domain.Company

	switch {
	// TODO: stop shoting your legs pls ogrn != ogrn
	case item.IndividualEntrepreneur != nil:
		company = domain.Company{
			OGRN:     item.IndividualEntrepreneur.OGRN,
			Contacts: item.IndividualEntrepreneur.Contacts,
			INN:      item.IndividualEntrepreneur.INN,
			CEO:      item.IndividualEntrepreneur.FullName,
			Finances: "",
		}
	case item.LegalEntity != nil:
		result := []string{}

		if v, ok := item.LegalEntity.Finances["Выручка"]; ok {
			result = append(result, fmt.Sprintf("Выручка: %s тыс.руб.", v))
		}
		if v, ok := item.LegalEntity.Finances["Год"]; ok {
			result = append(result, fmt.Sprintf("Год: %s", v))
		}
		company = domain.Company{
			OGRN:     item.LegalEntity.OGRN,
			Contacts: item.LegalEntity.Contacts,
			INN:      item.LegalEntity.INN,
			CEO:      item.LegalEntity.CEO.FullName,
			Finances: strings.Join(result, ", "),
		}
	default:
		slog.Error("Error while converting company")
	}

	return company
}

func dbData(ogrns []string) ([]domain.Company, error) {
	db, err := sql.Open("sqlite3", "file:./db/store.sqlite?cache=shared")

	if err != nil {
		slog.Error("Error while connecting db " + err.Error())
	}

	defer db.Close()

	rows, err := db.Query("SELECT ogrn, contacts, finances, inn, ceo FROM main.Companies")
	if err != nil {
		return nil, err
	}

	companies := []domain.Company{}
	for rows.Next() {
		company := domain.Company{}
		err := rows.Scan(
			&company.OGRN,
			&company.Contacts,
			&company.Finances,
			&company.INN,
			&company.CEO,
		)
		if err != nil {
			return nil, err
		}
		companies = append(companies, company)
	}

	return companies, nil
}

func getApiData(ogrns []string) []domain.Company {
	companies := make([]domain.Company, len(ogrns))

	apiData, err := apiCall(ogrns)
	if err != nil {
		slog.Error("Error while fetching api data: " + err.Error())
	}

	for _, item := range apiData.Items {
		company := convertToCompany(item)

		companies = append(companies, company)
	}
	// a little dangerous
	for _, ogrn := range ogrns {
		setCachedOgrn(ogrn)
	}

	save(companies)

	return companies
}

func getOgrns(rows [][]string) ([]string, error) {
	var needleIndex int
	// Get header row
	for _, headerRow := range rows[:1] {
		for i, colCell := range headerRow {
			if colCell == needle {
				needleIndex = i
				slog.Info(fmt.Sprintf("%s ogrn col finded, index is %v\n", colCell, needleIndex))
			}
		}
	}

	needleSlice := make([]string, 0, len(rows[1:]))

	// Get rest rows
	for _, row := range rows[1:] {
		needleSlice = append(needleSlice, row[needleIndex])
	}

	return needleSlice, nil
}

func readSource(fileName string) ([][]string, error) {
	f, err := excelize.OpenFile(fileName)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	defer func() {
		// Close the spreadsheet.
		if err := f.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	// Get all the rows in the Sheet1.
	data, err := f.GetRows(defaultSheet)
	if err != nil {
		return nil, err
	}

	var spacing int
	for _, v := range data {
		if rowlen := len(v); spacing <= rowlen {
			spacing = rowlen
		}
	}

	for i, row := range data {
		data[i] = append(row, make([]string, spacing-len(row))...)
	}

	return data, nil
}

func writeToFile(rows [][]string) {
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			slog.Error(err.Error())
		}
	}()
	// Create a new sheet.
	index, err := f.NewSheet(defaultSheet)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	// Set active sheet of the workbook.
	f.SetActiveSheet(index)

	for i, row := range rows {
		err := f.SetSheetRow(defaultSheet, "A"+strconv.Itoa(i+1), &row)
		if err != nil {
			slog.Error(err.Error())
		}
	}

	// Save spreadsheet by the given path.
	if err := f.SaveAs(result); err != nil {
		slog.Error(err.Error())
	}
}

func exit() {
	needsToexit = true
}
