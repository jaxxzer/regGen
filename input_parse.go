package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/choueric/clog"
)

const (
	TAG_CHIP = iota
	TAG_REG
	TAG_FIELD
	TAG_COMMENT
	TAG_OTHER
)

type lineItem struct {
	tag         int
	data        string
	fieldValStr string // only for filed line
}

func printRawLine(w io.Writer, tag int, line string) {
	switch tag {
	case TAG_CHIP:
		fmt.Println("[  CHIP ]", line)
	case TAG_REG:
		fmt.Println("[  REG  ]", line)
	case TAG_COMMENT:
		fmt.Println("[ COMNT ]", line)
	case TAG_FIELD:
		fmt.Println("[ FIELD ]", line)
	case TAG_OTHER:
		fmt.Println("[ OTHER ]", line)
	default:
		clog.Fatal("Unkonw type: " + line)
	}
}

func printTrimItems(w io.Writer, items []*lineItem) {
	for _, i := range items {
		switch i.tag {
		case TAG_CHIP:
			fmt.Fprintln(w, "[  CHIP ]", i.data)
		case TAG_REG:
			fmt.Fprintln(w, "[  REG  ]", i.data)
		case TAG_FIELD:
			fmt.Fprintf(w, "[ FIELD ] %s (%s)\n", i.data, i.fieldValStr)
		}
	}
}

type reg struct {
	name   string
	offset uint64
	fields []*field
}

func (r *reg) String() string {
	return fmt.Sprintf("\"%s\", %#x", r.name, r.offset)
}

type regJar struct {
	chip string
	regs []*reg
}

func (jar *regJar) String() string {
	var str bytes.Buffer
	fmt.Fprintf(&str, "CHIP: \"%s\"\n", jar.chip)
	for _, r := range jar.regs {
		fmt.Fprintln(&str, r)
		for _, f := range r.fields {
			fmt.Fprintln(&str, "   ", f)
		}
	}
	return str.String()
}

func readLine(r *bufio.Reader) (string, error) {
	str, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}

	str = strings.Trim(str, "\r\n")
	return str, nil
}

func lineItemNew(line string) (i *lineItem, err error) {
	sLine := strings.TrimSpace(line)
	var tag int
	if strings.Contains(sLine, "<CHIP>") || strings.Contains(sLine, "<chip>") {
		tag = TAG_CHIP
		i = &lineItem{tag: TAG_CHIP, data: sLine}
	} else if strings.Contains(sLine, "<REG>") || strings.Contains(sLine, "<reg>") {
		tag = TAG_REG
		i = &lineItem{tag: TAG_REG, data: sLine}
	} else if m, _ := regexp.MatchString(`\s*#`, sLine); m {
		tag = TAG_COMMENT
	} else {
		if strs, ok := validField(sLine); ok {
			tag = TAG_FIELD
			i = &lineItem{tag: TAG_FIELD, data: strs[0], fieldValStr: strs[1]}
		} else {
			tag = TAG_OTHER
			if len(line) != 0 {
				clog.Fatal("Invalid Format: [" + line + "]")
			}
		}
	}

	if debug {
		printRawLine(os.Stdout, tag, line)
	}

	return
}

func processChip(line string) (string, error) {
	strs := strings.Split(line, ":")
	if len(strs) != 2 {
		clog.Fatal("Invalid Format: [" + line + "]")
	}
	return strings.TrimSpace(strs[1]), nil
}

func processReg(line string) (*reg, error) {
	r := &reg{}
	strs := strings.Split(line, ":")
	if len(strs) != 2 {
		clog.Fatal("Invalid Format: [" + line + "]")
	} else {
		offset, err := strconv.ParseInt(strings.TrimSpace(strs[1]), 0, 64)
		if err != nil {
			clog.Error(line)
			return nil, err
		}
		r.offset = uint64(offset)
	}

	a := strings.IndexByte(line, '[')
	b := strings.IndexByte(line, ']')
	if a != -1 && b != -1 {
		r.name = strings.TrimSpace(line[a+1 : b])
	} else {
		r.name = strconv.FormatUint(r.offset, 10)
	}
	return r, nil
}

func trim(reader *bufio.Reader) ([]*lineItem, error) {
	items := make([]*lineItem, 0)
	for {
		line, err := readLine(reader)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return nil, err
			}
		}

		i, err := lineItemNew(line)
		if err != nil {
			clog.Fatal(err)
		} else {
			if i != nil {
				items = append(items, i)
			}
		}
	}

	if debug {
		fmt.Println("----------------- after trim ---------------")
		printTrimItems(os.Stdout, items)
	}

	return items, nil
}

func parse(items []*lineItem) (*regJar, error) {
	var curReg *reg
	jar := &regJar{}
	for _, item := range items {
		switch item.tag {
		case TAG_CHIP:
			chip, err := processChip(item.data)
			if err != nil {
				return nil, err
			}
			jar.chip = chip
		case TAG_REG:
			r, err := processReg(item.data)
			if err != nil {
				return nil, err
			}
			jar.regs = append(jar.regs, r)
			curReg = r
		case TAG_FIELD:
			if curReg == nil {
				clog.Fatal("Invalid Format: no <REG> at start")
			}
			f, err := processFiled(item.data, item.fieldValStr)
			if err != nil {
				return nil, err
			}
			curReg.fields = append(curReg.fields, f)
		}
	}

	if debug {
		fmt.Println("----------------- after parse ---------------")
		fmt.Println(jar)
	}

	return jar, nil
}

func regJarNew(filename string) (*regJar, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// lines in file ---> []lineItem
	items, err := trim(bufio.NewReader(f))
	if err != nil {
		return nil, err
	}

	// []lineItem -> regJar
	jar, err := parse(items)
	if err != nil {
		return nil, err
	}

	return jar, nil
}
