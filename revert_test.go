package p4

import (
	"fmt"
	"os"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestConn_Revert(t *testing.T) {
	var (
		err  error
		conn *Conn
	)

	conn, err = setup(t)
	Convey("test Revert", t, func() {
		So(err, ShouldBeNil)

		// 查询stream
		stream, err := conn.ChangeListStream(15585)
		So(stream, ShouldNotBeEmpty)
		So(err, ShouldBeNil)

		// 创建临时partitioned workspace
		streamWs := strings.Trim(stream, "/")

		// root_DM99.ZGame.Project-Development-xiner_test
		client := "root" + "_" + strings.ReplaceAll(streamWs, "/", "-")
		wsRoot, _ := os.Getwd()
		clientInfo := Client{
			Client:        client,
			Owner:         "root",
			Root:          wsRoot + "/" + client,
			Options:       "noallwrite noclobber nocompress unlocked nomodtime normdir",
			SubmitOptions: "submitunchanged",
			Stream:        stream,
			View:          []string{fmt.Sprintf("%s/... //%s/...", stream, client)},
		}
		message, err := conn.CreatePartitionClient(clientInfo)
		So(message, ShouldNotBeEmpty)
		// Client root_DM99.ZGame.Project-Development-xiner_test saved.
		//So(message, ShouldEqual, fmt.Sprintf("Client %s saved.", client))
		So(err, ShouldBeNil)

		conn = conn.WithClient(client)

		Convey("Describe Shelved", func() {
			message, err = conn.Revert(15585)
			So(message, ShouldNotBeNil)
			So(err, ShouldBeNil)
		})
	})
}
