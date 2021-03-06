package compressToolkit

import (
	"archive/zip"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/StevenZack/tools/strToolkit"
)

func CompressFilesTo(dst io.Writer, paths []string, progress func(offset, total int64) error) error {
	const bufSize = 32 << 10
	var total, offset int64
	// total
	for _, path := range paths {
		info, e := os.Stat(path)
		if e != nil {
			return e
		}
		if info.IsDir() {
			e = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
				if !info.IsDir() {
					total += info.Size()
				}
				return nil
			})
			if e != nil {
				log.Println(e)
				return e
			}
		} else {
			total += info.Size()
		}
	}

	// write
	zw := zip.NewWriter(dst)
	defer zw.Close()
	for _, root := range paths {
		info, e := os.Stat(root)
		if e != nil {
			return e
		}
		base := filepath.Dir(root)
		if info.IsDir() {
			e = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				rel, e := filepath.Rel(base, path)
				if e != nil {
					log.Println(e)
					return e
				}
				if info.IsDir() {
					_, e := zw.Create(strToolkit.Getrpath(rel))
					if e != nil {
						log.Println(e)
						return e
					}
					return nil
				}
				header, e := zip.FileInfoHeader(info)
				if e != nil {
					log.Println(e)
					return e
				}
				header.Name = rel

				writer, e := zw.CreateHeader(header)
				if e != nil {
					log.Println(e)
					return e
				}
				fi, e := os.OpenFile(path, os.O_RDONLY, 0644)
				if e != nil {
					log.Println(e)
					return e
				}
				defer fi.Close()
				buf := make([]byte, bufSize)
				for {
					n, e := fi.Read(buf)
					if e != nil {
						if e == io.EOF {
							break
						}
						log.Println(e)
						return e
					}
					_, e = writer.Write(buf[:n])
					if e != nil {
						log.Println(e)
						return e
					}
					offset += int64(n)
					if progress != nil {
						progress(offset, total)
					}
				}
				return nil
			})
			if e != nil {
				log.Println(e)
				return e
			}
			continue
		}
		// is file
		header, e := zip.FileInfoHeader(info)
		if e != nil {
			log.Println(e)
			return e
		}
		writer, e := zw.CreateHeader(header)
		if e != nil {
			log.Println(e)
			return e
		}
		fi, e := os.OpenFile(root, os.O_RDONLY, 0644)
		if e != nil {
			log.Println(e)
			return e
		}
		defer fi.Close()
		buf := make([]byte, bufSize)
		for {
			n, e := fi.Read(buf)
			if e != nil {
				if e == io.EOF {
					break
				}
				log.Println(e)
				return e
			}
			_, e = writer.Write(buf[:n])
			if e != nil {
				log.Println(e)
				return e
			}
			offset += int64(n)
			if progress != nil {
				progress(offset, total)
			}
		}
	}
	progress(total, total)
	return nil
}

func CompressFileTo(dst io.Writer, path string, progress func(offset, total int64)) error {
	const bufSize = 32 * 1024
	var total int64
	info, e := os.Stat(path)
	if e != nil {
		return e
	}
	if !info.IsDir() {
		total = info.Size()
		file, e := os.OpenFile(path, os.O_RDONLY, 0644)
		if e != nil {
			return e
		}
		defer file.Close()

		zw := zip.NewWriter(dst)
		defer zw.Close()
		header, e := zip.FileInfoHeader(info)
		if e != nil {
			return e
		}
		writer, e := zw.CreateHeader(header)
		if e != nil {
			return e
		}

		buf := make([]byte, bufSize)
		var offset int64
		for {
			n, e := file.Read(buf)
			if e != nil {
				if e == io.EOF {
					break
				}
				return e
			}
			_, e = writer.Write(buf[:n])
			if e != nil {
				return e
			}
			offset += int64(n)
			if progress != nil {
				progress(offset, total)
			}
		}
		return nil
	}

	e = filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	if e != nil {
		return e
	}

	zw := zip.NewWriter(dst)
	defer zw.Close()
	var offset int64
	base := strToolkit.SubBeforeLast(strToolkit.Getunpath(path), string(os.PathSeparator), strToolkit.Getrpath(path)) + string(os.PathSeparator)

	e = filepath.Walk(path, func(item string, info os.FileInfo, e error) error {
		if info.IsDir() {
			return nil
		}
		header, e := zip.FileInfoHeader(info)
		if e != nil {
			return e
		}
		header.Name = item[len(base):]
		writer, e := zw.CreateHeader(header)
		if e != nil {
			return e
		}
		buf := make([]byte, 32*1024)
		file, e := os.OpenFile(item, os.O_RDONLY, 0644)
		if e != nil {
			return e
		}
		defer file.Close()
		for {
			n, e := file.Read(buf)
			if e != nil {
				if e == io.EOF {
					break
				}
				return e
			}
			_, e = writer.Write(buf[:n])
			if e != nil {
				return e
			}
			offset += int64(n)
			if progress != nil {
				progress(offset, total)
			}
		}
		return nil
	})
	return nil
}

//解压
func Decompress(zipFile, dest string) error {
	reader, e := zip.OpenReader(zipFile)
	if e != nil {
		log.Println(e)
		return e
	}
	defer reader.Close()
	for _, file := range reader.File {
		filename := strToolkit.Getrpath(dest) + file.Name
		rc, e := file.Open()
		if e != nil {
			log.Println(e)
			return e
		}
		defer rc.Close()
		if file.FileInfo().IsDir() {
			os.MkdirAll(filename, 0755)
			continue
		}
		e = os.MkdirAll(getDir(filename), 0755)
		if e != nil {
			log.Println(e)
			return e
		}
		w, e := os.Create(filename)
		if e != nil {
			log.Println(e)
			return e
		}
		defer w.Close()
		_, e = io.Copy(w, rc)
		if e != nil {
			log.Println(e)
			return e
		}
	}
	return nil
}

func getDir(path string) string {
	return subString(path, 0, strings.LastIndex(path, "/"))
}

func subString(str string, start, end int) string {
	rs := []rune(str)
	length := len(rs)

	if start < 0 || start > length {
		panic("start is wrong")
	}

	if end < start || end > length {
		panic("end is wrong")
	}

	return string(rs[start:end])
}
