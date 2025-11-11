package main

import (
	"bufio"
	_ "embed"
	"encoding/binary"
	"fmt"
	"io"
	"path/filepath"

	// "io/fs"
	"os"
	"path"
	"strings"
)

type hlist = map[uint32]string

type zdir2002 struct {
	NameHash    uint32
	LocalOffset uint32
	Size        uint32
}

//go:embed files.list
var fileList string

func main() {
	argc := len(os.Args)
	if argc < 3 {
		printHelp()
	}

	headers, err := loadHeaders(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load headers: %v\n", err)
		os.Exit(1)
	}

	var outPath string
	hashList := loadHashList(&fileList)
	for _, header := range headers {
		name, ok := hashList[header.NameHash]
		normalized := filepath.FromSlash(strings.ReplaceAll(name, `\`, `/`))

		if ok {
			outPath = path.Join("EXTRACTED", normalized)
		} else {
			outPath = path.Join("EXTRACTED", "__UNKNOWN__", fmt.Sprintf("%X", header.LocalOffset))
		}
		err := extractFile(os.Args[2], outPath, int64(header.LocalOffset<<11), int64(header.Size))
		fmt.Println(outPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}

	os.Exit(0)
}

func extractFile(archivePath, outPath string, offset, size int64) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	// Make sure parent dir exists
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// SectionReader reads only the chunk we care about
	section := io.NewSectionReader(archive, offset, size)

	// Optional: use a custom buffer size
	buf := make([]byte, 32*1024) // 32 KB chunks
	_, err = io.CopyBuffer(outFile, section, buf)
	return err
}

func printHelp() {
	fmt.Printf("Usage: %s <ZDIR> <ZZDATA>\n", path.Base(os.Args[0]))
	os.Exit(1)
}

func loadHashList(list *string) hlist {
	hashList := make(hlist)
	reader := strings.NewReader(*list)
	scanner := bufio.NewScanner(reader)

	for scanner.Scan() {
		name := scanner.Text()
		hash := getFileNamehash(&name)
		hashList[hash] = name
	}
	return hashList
}

func getFileNamehash(name *string) uint32 {
	hash := uint32(0xFFFFFFFF)
	for i := range len(*name) {
		hash = 33*hash + uint32(rune((*name)[i]))
	}
	return hash
}

func loadHeaders(name string) ([]zdir2002, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fInfo, err := f.Stat()
	if err != nil {
		return nil, err
	}

	headers := make([]zdir2002, fInfo.Size()/12)
	if err := binary.Read(f, binary.LittleEndian, &headers); err != nil {
		return nil, err
	}

	return headers, nil
}
