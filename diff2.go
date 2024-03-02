package p4

import (
	"encoding/json"
	"strconv"
)

type DiffFile struct {
	DepotFile string `json:"depotFile"`
	Revision  uint64 `json:"revision"` // content时，版本会不一致
	Type      string `json:"type"`     // text,binary+l
}

type Diff2 struct {
	Code      string    `json:"code"`   // stat
	Status    string    `json:"status"` // identical(一致), content(有差异)
	DiffFile1 *DiffFile `json:"diffFile1"`
	DiffFile2 *DiffFile `json:"diffFile2"`
}

func (diff *Diff2) String() string {
	buf, _ := json.Marshal(diff)
	return string(buf)
}

// Diff2 参考手册：
// https://www.perforce.com/manuals/cmdref/Content/CmdRef/p4_diff2.html#p4_diff2
func (conn *Conn) Diff2(myStreamSpec, yourStreamSpec string) (diffs []*Diff2, err error) {
	var (
		results []Result
	)
	if results, err = conn.RunMarshaled("diff2", []string{myStreamSpec, yourStreamSpec}); err != nil {
		return
	}
	for idx := range results {
		if diff, ok := results[idx].(*Diff2); !ok {
			continue
		} else {
			diffs = append(diffs, diff)
		}
	}
	return
}

func (conn *Conn) Diff2Change(myStream string, myChange uint64, yourStream string, yourChange uint64) ([]*Diff2, error) {
	return conn.Diff2(myStream+"@"+strconv.FormatUint(myChange, 10), yourStream+"@"+strconv.FormatUint(yourChange, 10))
}

func (conn *Conn) Diff2Shelve(myStream string, myShelve uint64, yourStream string, yourShelve uint64) ([]*Diff2, error) {
	return conn.Diff2(myStream+"@="+strconv.FormatUint(myShelve, 10), yourStream+"@="+strconv.FormatUint(yourShelve, 10))
}
