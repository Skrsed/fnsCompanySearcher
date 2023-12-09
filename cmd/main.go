package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"github.com/xuri/excelize/v2"
)

func main() {
	ReadFile()
}

func ReadRawFile() {
	file, err := os.Open("../source.xlsx")

	if err != nil {
		log.Fatalf("error on opening file: %v\n", err)
	}
	defer file.Close()

	lines := make([]string, 0, 30000)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	fmt.Println(len(lines), lines[len(lines)-2])
}

func ReadFile() {
	file, err := excelize.OpenFile("../source.xlsx")
	i := 0
	if err != nil {
		log.Fatalf("error on opening file: %v\n", err)
		os.Exit(1)
	}

	defer func() {
		if err := file.Close(); err != nil {
			log.Fatalf("error on closing the stream: %v\n", err)
		}
	}()

	rows, err := file.Rows("Лист1")
	if err != nil {
		log.Fatalf("error on reading rows %v\n", err)
		return
	}

	for rows.Next() {
		row, err := rows.Columns()
		if err != nil {
			log.Println(err)
		}
		for _, colCell := range row {
			fmt.Sprint(colCell)
			i++
		}

	}
	defer rows.Close()

	fmt.Println(i)
}
