package excel

import (
	"fmt"

	"github.com/xuri/excelize/v2"
)

type ExcelRepo struct {
	file *excelize.File
}

func (e *ExcelRepo) Close() {
	if err := e.file.Close(); err != nil {
		fmt.Println(err)
	}
}

func NewExcelRepo() (ExcelRepo, error) {
	file, err := excelize.OpenFile("Book1.xlsx")
	if err != nil {
		return ExcelRepo{}, nil
	}

	return ExcelRepo{
		file,
	}, nil
}

func (e *ExcelRepo) ReadNext() {
	rows, err := e.file.Rows("Sheet1")
	if err != nil {
		fmt.Println(err)
		return
	}

}
