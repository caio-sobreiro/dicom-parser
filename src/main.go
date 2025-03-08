package main

import (
	"fmt"
	"os"

	"dicom-parser/src/internal/services"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: dcmparse <dicomfile>")
		os.Exit(1)
	}

	dicomfile := os.Args[1]

	file, err := os.Open(dicomfile)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer file.Close()

	dicomService := services.NewDicomService()
	err = dicomService.ParseDicomFile(file)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
