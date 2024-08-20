package p4

import (
	"bytes"
	"encoding/json"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"golang.org/x/exp/slices"
)

type StreamInfo struct {
	Stream  string
	Owner   string
	Name    string
	Parent  string // steam type为mainline时，parent必须为none，其余类型stream需要填写现有的stream（格式：//depotname/streamname）
	Type    string // mainline, development, release, virtual, task
	Options string // [all|owner]submit, [un]locked, [no]toparent, [no]fromparent, mergeany|mergedown
	Paths   []string
}

const (
	OwnerSubmit, Locked, ToParent, FromParent, MergeAny      = 0, 0, 0, 0, 0
	AllSubmit, UnLocked, NoToParent, NoFromParent, MergeDown = 1, 1, 1, 1, 1
)

var (
	streamOptsMapping = [][]string{
		{"ownersubmit", "allsubmit"}, // checked, unchecked
		{"locked", "unlocked"},
		{"toparent", "notoparent"},
		{"fromparent", "nofromparent"},
		{"mergeany", "mergedown"},
	}
)

func (s *StreamInfo) String() string {
	buf, _ := json.Marshal(s)
	return string(buf)
}

func (conn *Conn) Streams() (list []*StreamInfo, err error) {
	var (
		result []Result
	)
	if result, err = conn.RunMarshaled("streams", []string{}); err != nil {
		return
	}
	if len(result) == 0 {
		return
	}
	for _, v := range result {
		if stream, ok := v.(*StreamInfo); !ok {
			return
		} else {
			list = append(list, stream)
		}
	}
	return
}

// Stream 入参：stream路径
func (conn *Conn) Stream(location string) (stream *StreamInfo, err error) {
	var (
		ok     bool
		result []Result
	)
	if result, err = conn.RunMarshaled("stream", []string{"-o", location}); err != nil {
		return
	}
	if len(result) == 0 {
		err = errors.Errorf("%s - no such stream.", location)
		return
	}
	if stream, ok = result[0].(*StreamInfo); !ok {
		err = errors.Errorf("%s - no such stream.", location)
		return
	}
	return
}

var _streamTemplate = template.New("stream template")
var _streamTemplateTxt = `Stream:  {{ .Stream }}
Owner:        {{ .Owner }}
Name:         {{ .Name }}
Parent:       {{ if eq .Type "mainline" }}none{{else}}{{ .Parent }}{{ end }}
Type:         {{ .Type }}
Description:
        Created by {{ .Owner }} automatically.
{{- if ne .Type "mainline" }}
Options:        {{ .Options }}
{{- end }}
Paths:
        share ...
`

type StreamOption func(si *StreamInfo) error

// WithOptions [all|owner]submit, [un]locked, [no]toparent, [no]fromparent, mergeany|mergedown
// A virtual stream must have its flow options set to notoparent and nofromparent.
// https://www.perforce.com/manuals/cmdref/Content/CmdRef/p4_stream.html#p4_stream
func WithOptions(options []int) StreamOption {
	return func(si *StreamInfo) error {
		if len(options) != len(streamOptsMapping) {
			return errors.Errorf("options count is invalid, must be '%d'", len(streamOptsMapping))
		}
		if si.Type == "virtual" && len(options) > 3 {
			options[2] = NoToParent
			options[3] = NoFromParent
		}
		var opts []string
		for idx, o := range options {
			values := streamOptsMapping[idx]
			opts = append(opts, values[o])
		}
		si.Options = strings.Join(opts, " ")
		return nil
	}
}

var _streamTypes []string

// CreateStream 创建分支
// mainline分支，parent填空，populate为false
// 其他有父分支的，populate为true，表示从父分支拷贝项目内容到新分支
func (conn *Conn) CreateStream(name, streamType, parent, location string, populate bool, options ...StreamOption) (message string, err error) {
	var (
		out        []byte
		contentBuf = bytes.NewBuffer(nil)
		streamInfo = StreamInfo{
			Stream:  location,
			Owner:   conn.username,
			Name:    name,
			Parent:  parent,
			Type:    streamType,
			Options: "allsubmit unlocked toparent fromparent mergedown",
		}
	)
	if streamType == "mainline" {
		populate = false
		streamInfo.Parent = "none"
	}
	if streamType == "virtual" {
		populate = false
		streamInfo.Options = "allsubmit unlocked notoparent nofromparent mergedown"
	}
	for _, fn := range options {
		if fn == nil {
			continue
		}
		if err = fn(&streamInfo); err != nil {
			return
		}
	}
	if !slices.Contains(_streamTypes, streamType) {
		err = errors.Errorf("streamType should be one of the following '%s'", strings.Join(_streamTypes, "', '"))
		return
	}
	if err = validateLocation(location); err != nil {
		return
	}
	if _, err = _streamTemplate.Parse(_streamTemplateTxt); err != nil {
		return
	}
	if err = _streamTemplate.Execute(contentBuf, streamInfo); err != nil {
		return
	}
	if out, err = conn.Input([]string{"stream", "-i"}, contentBuf.Bytes()); err != nil {
		return
	}
	message = strings.TrimSpace(string(out))
	if populate {
		if _, err = conn.Populate(location); err != nil {
			return
		}
	}
	return
}

// DeleteStream prune为true，将删除stream中的文件，慎用!
// location格式: //Stream_Root
func (conn *Conn) DeleteStream(location string, prune bool) (message string, err error) {
	var (
		out      []byte
		shelved  []*Change
		clients  []*Client
		unloaded []*Client
	)
	if err = validateLocation(location); err != nil {
		return
	}
	// 1. 删除Stream中所有Shelve的文件
	if shelved, err = conn.Shelved(location + "/..."); err != nil {
		return
	}
	for _, s := range shelved {
		if _, err = conn.DeleteShelved(location+"/...", s.Change); err != nil {
			return
		}
	}

	// 2. 删除Stream关联的所有Clients
	if clients, err = conn.Clients(location); err != nil {
		return
	}
	for _, c := range clients {
		if _, err = conn.DeleteClient(c.Client); err != nil {
			return
		}
	}
	// 3. 删除Stream关联的所有Unloaded Clients
	if unloaded, err = conn.UnloadedClients(location); err != nil {
		return
	}
	for _, c := range unloaded {
		if _, err = conn.DeleteClient(c.Client); err != nil {
			return
		}
	}

	// 4. 删除Stream中的所有文件
	if prune {
		if _, err = conn.Prune(location); err != nil {
			return
		}
	}

	// 5. 删除Stream Spec
	if out, err = conn.Output([]string{"stream", "-d", location}); err != nil {
		return
	}
	message = strings.TrimSpace(string(out))
	return
}

func validateLocation(location string) (err error) {
	if len(location) <= 0 {
		err = errors.New("仓库路径非法: 不能为空")
		return
	}
	if !strings.HasPrefix(location, "//") {
		err = errors.Errorf("仓库路径非法: '%s' 没有以 '//' 开头", location)
		return
	}
	if strings.HasSuffix(location, "/...") {
		err = errors.Errorf("仓库路径非法: '%s' 不能以 '/...' 结尾", location)
		return
	}
	return
}

func init() {
	_streamTypes = []string{"mainline", "development", "release", "virtual", "task"}
}
