package services

import (
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"dicom-parser/src/internal/types"
)

type DicomService struct{}

func NewDicomService() *DicomService {
	return &DicomService{}
}

func (d *DicomService) ParseDicomFile(dicomfile *os.File) error {
	// first 128 bytes are empty. Skip them.
	dicomfile.Seek(128, 0)

	// next 4 bytes should be "DICM". This confirms that the file is a DICOM file
	preamble := make([]byte, 4)
	dicomfile.Read(preamble)

	if string(preamble) != "DICM" {
		log.Fatal("Not a DICOM file")
	}

	transfersyntax := ""

	fmt.Println("Meta Header:")

	// read the meta header
	for {
		group, element := d.ReadGroupAndElement(dicomfile)

		// if not group 2, then we are done with the meta header
		if group != 0x0002 {
			// rewind 4 bytes
			dicomfile.Seek(-4, 1)
			break
		}

		// meta header is always explicit VR
		tag := d.ReadExplicitVRTag(dicomfile)
		tag.Group = group
		tag.Element = element

		// store Transfer Syntax UID for later
		if element == 0x0010 {
			transfersyntax = string(tag.Value)
		}

		d.PrintTag(tag, 0)
	}

	isExplicitVR := true
	// check if the transfer syntax is explicit VR
	if transfersyntax == "1.2.840.10008.1.2" {
		isExplicitVR = false
	}

	fmt.Println()
	fmt.Println("Data Set:")

	seqDepth := 0
	seqLengthRemaining := uint32(0)

	// read the rest of the file
	for {
		var tag *types.Tag
		group, element := d.ReadGroupAndElement(dicomfile)
		if group == 0 && element == 0 {
			break
		}

		// sequence item
		if group == 0xfffe && element == 0xe000 {
			seqDepth++

			length := make([]byte, 4)
			dicomfile.Read(length)
			lengthValue := binary.LittleEndian.Uint32(length)

			seqLengthRemaining -= lengthValue

			if seqLengthRemaining == 0 {
				seqDepth--
			}

			fmt.Printf("  (%04x,%04x) %s(%d) <Sequence Item>\n", group, element, "NA", lengthValue)
			continue
		}

		if isExplicitVR {
			tag = d.ReadExplicitVRTag(dicomfile)
		} else {
			tag = d.ReadImplicitVRTag(dicomfile)
		}

		tag.Group = group
		tag.Element = element

		// if pixel data
		// if group == 0x7fe0 && element == 0x0010 {
		// 	fmt.Printf("(7fe0,0010) %s(%d) <Pixel Data>\n", tag.VR, tag.Length)
		// 	continue
		// }

		// nested sequences
		if tag.VR == "SQ" {
			seqDepth++
			seqLengthRemaining = tag.Length
			fmt.Printf("(%04x,%04x) %s(%d) <Sequence of Items>\n", group, element, tag.VR, tag.Length)
			continue
		}

		d.PrintTag(tag, seqDepth)
	}

	return nil
}

func (d *DicomService) ReadGroupAndElement(file *os.File) (uint16, uint16) {
	groupBytes := make([]byte, 2)
	file.Read(groupBytes)
	group := binary.LittleEndian.Uint16(groupBytes)

	elementBytes := make([]byte, 2)
	file.Read(elementBytes)
	element := binary.LittleEndian.Uint16(elementBytes)

	return group, element
}

func (d *DicomService) ReadExplicitVRTag(file *os.File) *types.Tag {
	// next 2 bytes represent the type (VR) of the tag
	vrBytes := make([]byte, 2)
	file.Read(vrBytes)
	vr := string(vrBytes)

	// AE, AS, AT, CS, DA, DS, DT, FL, FD, IS, LO, LT, PN, SH, SL, SS, ST, TM, UI, UL and US
	var length uint32
	if slices.Contains([]string{"AE", "AS", "AT", "CS", "DA", "DS", "DT", "FL", "FD", "IS", "LO", "LT", "PN", "SH", "SL", "SS", "ST", "TM", "UI", "UL", "US"}, string(vrBytes)) {
		// next 2 bytes are the length of the value
		lengthBytes := make([]byte, 2)
		file.Read(lengthBytes)
		length = uint32(binary.LittleEndian.Uint16(lengthBytes))
	} else {
		// 2 reserved bytes
		file.Seek(2, 1)

		// next 4 bytes are the length of the value
		lengthBytes := make([]byte, 4)
		file.Read(lengthBytes)
		length = binary.LittleEndian.Uint32(lengthBytes)
	}

	// if it's a sequence, don't read all the bytes
	if vr == "SQ" {
		return &types.Tag{
			VR:     vr,
			Length: length,
		}
	}

	// next `length` bytes are the value, in little endian
	valueBytes := make([]byte, length)
	file.Read(valueBytes)

	tag := &types.Tag{
		VR:     vr,
		Length: length,
		Value:  valueBytes,
	}

	return tag
}

func (d *DicomService) ReadImplicitVRTag(file *os.File) *types.Tag {
	// next 4 bytes are the length of the value
	lengthBytes := make([]byte, 4)
	file.Read(lengthBytes)
	length := binary.LittleEndian.Uint32(lengthBytes)

	// next `length` bytes are the value, in little endian
	valueBytes := make([]byte, length)
	file.Read(valueBytes)

	tag := &types.Tag{
		Length: length,
		Value:  valueBytes,
	}

	return tag
}

/*
This is a "hack" to guess the correct type of the tag based on the length of the value.
*/
func (d *DicomService) PrintTag(tag *types.Tag, depth int) {
	fmt.Printf(strings.Repeat(" ", depth*2))
	fmt.Printf("(%04x,%04x) %s(%d) ", tag.Group, tag.Element, tag.VR, tag.Length)

	if tag.Length > 1024 {
		fmt.Printf("<Value is too long to display>\n")
		return
	}

	switch tag.Length {
	case 1:
		fmt.Printf("%d\n", tag.Value[0])
	case 2:
		fmt.Printf("%d\n", binary.LittleEndian.Uint16(tag.Value))
	case 4:
		fmt.Printf("%d\n", binary.LittleEndian.Uint32(tag.Value))
	case 8:
		fmt.Printf("%d\n", binary.LittleEndian.Uint64(tag.Value))
	default:
		fmt.Printf("%s\n", string(tag.Value))
	}
}
