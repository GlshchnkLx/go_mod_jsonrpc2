package jsonrpc

import (
	"encoding/json"
	"fmt"
	"reflect"
)

type Request struct {
	Jsonrpc string          `json:"jsonrpc"`
	Id      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	Jsonrpc string          `json:"jsonrpc"`
	Id      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Error struct {
	Code    int32       `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e Error) Error() string {
	return fmt.Sprintf(`code: %d; message: "%s"`, e.Code, e.Message)
}

type HandlerFunc func(interface{}) (interface{}, error)

type Handler struct {
	Request  reflect.Type
	Response reflect.Type
	Function HandlerFunc
}
