package main

import (
	"bufio"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

//go:embed files.list
var embeddedFileList string

const (
	// z7d2            = 12
	// z7d3            = 12
	z2002           = 12
	z2003           = 24
	extractedRoot   = "EXTRACTED"
	unknownDir      = "__UNKNOWN__"
	bufferSize      = 32 * 1024
	headerSizeBytes = 12
	offsetShift     = 11
)

type zdir2002 struct {
	NameHash    uint32
	LocalOffset uint32
	Size        uint32
}

type zdir2003 struct {
	NameHash    uint32
	ArchiveID   uint32
	LocalOffset uint32
	TotalOffset uint32
	Size        uint32
	Checksum    uint32
}

type header struct {
	NameHash    uint32
	LocalOffset uint32
	Size        uint32
}

type hlist map[uint32]string

func detectZdirType(zname string) (int, error) {
	f, err := os.Open(zname)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return 0, err
	}

	size := stat.Size()

	if size%z2003 == 0 {
		return z2003, nil
	} else if size%z2002 == 0 {
		return z2002, nil
	}

	return 0, errors.New("invalid 'ZDIR' file size")
}

func main() {
	if len(os.Args) < 3 {
		printUsage()
	}

	headerPath := os.Args[1]
	archivePath := os.Args[2]

	headers, err := loadHeaders(headerPath)
	if err != nil {
		exitWithError("failed to load headers", err)
	}

	hashList := loadHashList(embeddedFileList)

	for _, hdr := range headers {
		outPath := buildOutputPath(hdr, hashList)
		if err := extractFile(archivePath, outPath, int64(hdr.LocalOffset)<<offsetShift, int64(hdr.Size)); err != nil {
			exitWithError(outPath, err)
		}
		fmt.Println(outPath)
	}
}

func printUsage() {
	progname := filepath.Base(os.Args[0])
	fmt.Printf("Usage: %s <ZDIR> <ZZDATA>\n", progname)
	fmt.Printf("Usage: %s <ZDIR> <ZZDATA0> <ZZDATA1> <ZZDATA2> ...\n", progname)
	fmt.Printf("Usage: %s <ZDIR> <ZZDATA{0..3}> ...\n", progname)
	os.Exit(1)
}

func loadZDIR[T any](zname string) ([]T, error) {
	f, err := os.Open(zname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	headers := make([]T, info.Size()/headerSizeBytes)
	if err := binary.Read(f, binary.LittleEndian, &headers); err != nil {
		return nil, err
	}
	return headers, nil
}

// func extractZ2002(zname, zzname string) error {
// }

// func extractZ2003(zname, zzname string) error {
// }

func buildOutputPath(h header, hashList hlist) string {
	if name, ok := hashList[h.NameHash]; ok {
		normalized := filepath.FromSlash(strings.ReplaceAll(name, `\`, `/`))
		return filepath.Join(extractedRoot, normalized)
	}
	return filepath.Join(extractedRoot, unknownDir, fmt.Sprintf("%X", h.LocalOffset))
}

func extractFile(archivePath, outPath string, offset, size int64) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	section := io.NewSectionReader(archive, offset, size)
	buf := make([]byte, bufferSize)

	if _, err := io.CopyBuffer(outFile, section, buf); err != nil {
		return err
	}

	return nil
}

func loadHeaders(path string) ([]header, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	if info.Size()%headerSizeBytes != 0 {
		return nil, errors.New("invalid header file size")
	}

	headers := make([]header, info.Size()/headerSizeBytes)
	if err := binary.Read(f, binary.LittleEndian, &headers); err != nil {
		return nil, fmt.Errorf("read headers: %w", err)
	}
	return headers, nil
}

func loadHashList(list string) hlist {
	hashes := make(hlist)
	scanner := bufio.NewScanner(strings.NewReader(list))

	for scanner.Scan() {
		name := scanner.Text()
		hash := getFileNameHash(name)
		hashes[hash] = name
	}
	return hashes
}

func getFileNameHash(name string) uint32 {
	hash := uint32(0xFFFFFFFF)
	for i := range len(name) {
		hash = 33*hash + uint32(name[i])
	}
	return hash
}

func exitWithError(context string, err error) {
	fmt.Fprintf(os.Stderr, "Error: %s: %v\n", context, err)
	os.Exit(1)
}
