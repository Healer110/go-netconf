// Go NETCONF Client
//
// Copyright (c) 2013-2018, Juniper Networks, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package netconf

import (
	"bytes"
	"crypto/rand"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const (
	editConfigXml = `<edit-config>
<target><%s/></target>
<default-operation>merge</default-operation>
<error-option>rollback-on-error</error-option>
<config>%s</config>
</edit-config>`
)

// RPCMessage represents an RPC Message to be sent.
type RPCMessage struct {
	MessageID string
	Methods   []RPCMethod
}

// NewRPCMessage generates a new RPC Message structure with the provided methods
func NewRPCMessage(methods []RPCMethod) *RPCMessage {
	return &RPCMessage{
		MessageID: msgID(),
		Methods:   methods,
	}
}

// MarshalXML marshals the NETCONF XML data
func (m *RPCMessage) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	var buf bytes.Buffer
	for _, method := range m.Methods {
		buf.WriteString(method.MarshalMethod())
	}

	data := struct {
		MessageID string `xml:"message-id,attr"`
		Xmlns     string `xml:"xmlns,attr"`
		Methods   []byte `xml:",innerxml"`
	}{
		m.MessageID,
		"urn:ietf:params:xml:ns:netconf:base:1.0",
		buf.Bytes(),
	}

	// Wrap the raw XML (data) into <rpc>...</rpc> tags
	start.Name.Local = "rpc"
	return e.EncodeElement(data, start)
}

// RPCReply defines a reply to a RPC request
type RPCReply struct {
	XMLName   xml.Name   `xml:"rpc-reply"`
	Errors    []RPCError `xml:"rpc-error,omitempty"`
	Data      string     `xml:",innerxml"`
	Ok        bool       `xml:",omitempty"`
	RawReply  string     `xml:"-"`
	MessageID string     `xml:"-"`
}

func newRPCReply(rawXML []byte, ErrOnWarning bool, messageID string) (*RPCReply, error) {
	reply := &RPCReply{}
	// reply.RawReply = string(rawXML)
	/*
		2025/02/06 修改源代码，
		分帧机制：
		NETCONF 1.1 引入了分帧机制，以解决大数据传输时的性能问题。
		NETCONF 1.0 使用 ]]>]]> 作为消息结束标记，而 NETCONF 1.1 使用 # 分帧机制来标记消息的长度。
		这使得 NETCONF 1.1 能够更高效地处理大数据传输，并减少消息解析的复杂性
		这里新增一个函数，用来将返回的数据去除chunk
	*/
	reply.RawReply = ProcessChunkedFraming(string(rawXML))
	rawXML = []byte(reply.RawReply)

	/*
		下面的xml.Unmarshal()使用的是函数中传递进来的原始数据，包含trunk fram
		所以上面事先做了处理，不然会报错如下
		XML syntax error on line 18: expected attribute name in element
	*/
	if err := xml.Unmarshal(rawXML, reply); err != nil {
		return nil, err
	}

	// will return a valid reply so setting Requests message id
	reply.MessageID = messageID

	if reply.Errors != nil {
		for _, rpcErr := range reply.Errors {
			if rpcErr.Severity == "error" || ErrOnWarning {
				return reply, &rpcErr
			}
		}
	}

	/*
		2025/02/06 修改源代码:
		Ok        bool       `xml:",omitempty"`
		在解析xml字符串时，处理<ok/>标签会有问题，这里重新进行判断，
		方便在主程序中利用该变量
	*/
	if strings.Contains(reply.RawReply, "<ok") {
		reply.Ok = true
	} else {
		reply.Ok = false
	}

	return reply, nil
}

// RPCError defines an error reply to a RPC request
type RPCError struct {
	Type     string `xml:"error-type"`
	Tag      string `xml:"error-tag"`
	Severity string `xml:"error-severity"`
	Path     string `xml:"error-path"`
	Message  string `xml:"error-message"`
	Info     string `xml:",innerxml"`
}

// Error generates a string representation of the provided RPC error
func (re *RPCError) Error() string {
	return fmt.Sprintf("netconf rpc [%s] '%s'", re.Severity, re.Message)
}

// RPCMethod defines the interface for creating an RPC method.
type RPCMethod interface {
	MarshalMethod() string
}

// RawMethod defines how a raw text request will be responded to
type RawMethod string

// MarshalMethod converts the method's output into a string
func (r RawMethod) MarshalMethod() string {
	return string(r)
}

// MethodLock files a NETCONF lock target request with the remote host
func MethodLock(target string) RawMethod {
	return RawMethod(fmt.Sprintf("<lock><target><%s/></target></lock>", target))
}

// MethodUnlock files a NETCONF unlock target request with the remote host
func MethodUnlock(target string) RawMethod {
	return RawMethod(fmt.Sprintf("<unlock><target><%s/></target></unlock>", target))
}

// MethodGetConfig files a NETCONF get-config source request with the remote host
func MethodGetConfig(source string) RawMethod {
	return RawMethod(fmt.Sprintf("<get-config><source><%s/></source></get-config>", source))
}

// MethodGet files a NETCONF get source request with the remote host
func MethodGet(filterType string, dataXml string) RawMethod {
	return RawMethod(fmt.Sprintf("<get><filter type=\"%s\">%s</filter></get>", filterType, dataXml))
}

// MethodEditConfig files a NETCONF edit-config request with the remote host
func MethodEditConfig(database string, dataXml string) RawMethod {
	return RawMethod(fmt.Sprintf(editConfigXml, database, dataXml))
}

var msgID = uuid

// uuid generates a "good enough" uuid without adding external dependencies
func uuid() string {
	b := make([]byte, 16)
	io.ReadFull(rand.Reader, b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}
