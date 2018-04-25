package regjar

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/choueric/clog"
	"github.com/choueric/goutils"
	"github.com/choueric/regGen/dbg"
)

type Reg struct {
	Name   string
	Offset uint64
	Fields []*Field
}

func (r *Reg) String() string {
	return fmt.Sprintf("\"%s\", %#x", r.Name, r.Offset)
}

func (r *Reg) addFileds(f ...*Field) {
	r.Fields = append(r.Fields, f...)
}

type Jar struct {
	Chip  string
	Width uint32
	Regs  []*Reg
}

func (jar *Jar) String() string {
	var str bytes.Buffer
	fmt.Fprintf(&str, "CHIP: \"%s\"\n", jar.Chip)
	fmt.Fprintf(&str, "WIDTH: %d\n", jar.Width)
	for _, r := range jar.Regs {
		fmt.Fprintln(&str, r)
		for _, f := range r.Fields {
			fmt.Fprintln(&str, "   ", f)
		}
	}
	return str.String()
}

func (jar *Jar) addRegs(v ...*Reg) {
	jar.Regs = append(jar.Regs, v...)
}

// create a new jar and setup default values
func newJar() *Jar {
	jar := new(Jar)
	jar.Width = 8
	return jar
}

func processChip(line string) (string, error) {
	strs := strings.Split(line, ":")
	if len(strs) != 2 {
		clog.Fatal("Invalid Format: [" + line + "]")
	}
	return strings.TrimSpace(strs[1]), nil
}

func processWidth(line string) (uint32, error) {
	strs := strings.Split(line, ":")
	if len(strs) != 2 {
		clog.Fatal("Invalid Format: [" + line + "]")
	}

	w, err := goutils.ParseUint(strings.TrimSpace(strs[1]), 32)
	if err != nil {
		return 0, err
	}
	return uint32(w), nil
}

func processReg(line string) (*Reg, error) {
	r := new(Reg)
	strs := strings.Split(line, ":")
	if len(strs) != 2 {
		clog.Fatal("Invalid Format: [" + line + "]")
	} else {
		offset, err := strconv.ParseInt(strings.TrimSpace(strs[1]), 0, 64)
		if err != nil {
			clog.Error(line)
			return nil, err
		}
		r.Offset = uint64(offset)
	}

	a := strings.IndexByte(line, '[')
	b := strings.IndexByte(line, ']')
	if a != -1 && b != -1 {
		r.Name = strings.TrimSpace(line[a+1 : b])
	} else {
		r.Name = strconv.FormatUint(r.Offset, 10)
	}
	return r, nil
}

func trim(reader *bufio.Reader) (tagItemSlice, error) {
	items := newTagItemSlice()
	for {
		line, err := goutils.ReadLine(reader)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return nil, err
			}
		}

		item := newTagItem(line)
		if err != nil {
			clog.Fatal(err)
		} else {
			if item != nil && item.tag != tag_comment {
				items.addTagItems(item)
			}
		}
	}

	if dbg.True {
		fmt.Println("----------------- after trim ---------------")
		fmt.Println(items)
	}

	return items, nil
}

func parse(items tagItemSlice) (*Jar, error) {
	var curReg *Reg
	jar := newJar()
	for _, item := range items {
		switch item.tag {
		case tag_chip:
			chip, err := processChip(item.data)
			if err != nil {
				return nil, err
			}
			jar.Chip = chip
		case tag_width:
			width, err := processWidth(item.data)
			if err != nil {
				return nil, err
			}
			jar.Width = width
		case tag_reg:
			r, err := processReg(item.data)
			if err != nil {
				return nil, err
			}
			jar.addRegs(r)
			curReg = r
		case tag_field:
			if curReg == nil {
				clog.Fatal("Invalid Format: no <REG> at start.")
			}
			f, err := processFiled(item.data, item.enums)
			if err != nil {
				return nil, err
			}
			if f.End >= jar.Width {
				clog.Fatal(fmt.Sprintf("Field offset %d is over bit-width %d.",
					f.End, jar.Width))
			}
			curReg.addFileds(f)
		}
	}

	if dbg.True {
		fmt.Println("----------------- after parse ---------------")
		fmt.Println(jar)
	}

	return jar, nil
}

func New(filename string) (*Jar, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// lines in file ---> tagItemSlice
	items, err := trim(bufio.NewReader(f))
	if err != nil {
		return nil, err
	}

	// tagItemSlice -> Jar
	jar, err := parse(items)
	if err != nil {
		return nil, err
	}

	return jar, nil
}