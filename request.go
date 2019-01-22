package jsonrpc

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strconv"
)

// Request interface
type Request interface {
	Version() string
	Method() string
	Params() interface{}
	ID() interface{}
	State(key string) interface{}

	// Serialization
	fmt.Stringer
	Bytes() []byte
}

// State can be optionally provided with Handle requests to pass extra state to
// the handler for that individual request.
type State map[string]interface{}

// Responder inteface
type Responder interface {
	NewSuccessResponse(result interface{}) Response
	NewErrorResponse(code int, message string) Response
	NewServerErrorResponse(err error) Response
}

// RequestResponder interface
type RequestResponder interface {
	Request
	Responder
}

// A JSON-RPC request object.
type request struct {
	RequestVersion string      `json:"jsonrpc"`
	RequestMethod  string      `json:"method"`
	RequestParams  interface{} `json:"params,omitempty"`
	RequestID      interface{} `json:"id"`
	requestState   State
}

// Version get the version
func (request *request) Version() string {
	return request.RequestVersion
}

// Method get the method
func (request *request) Method() string {
	return request.RequestMethod
}

// Params get the params
func (request *request) Params() interface{} {
	return request.RequestParams
}

// ID get id from request
func (request *request) ID() interface{} {
	return request.RequestID
}

// State get state from key
func (request *request) State(key string) interface{} {
	return request.requestState[key]
}

// NewSuccessResponse new success response
func (request *request) NewSuccessResponse(result interface{}) Response {
	return NewSuccessResponse(request.ID(), result)
}

// NewErrorResponse new error response
func (request *request) NewErrorResponse(code int, message string) Response {
	return NewErrorResponse(request.ID(), code, message)
}

// NewServerErrorResponse new server error response
func (request *request) NewServerErrorResponse(err error) Response {
	return NewServerErrorResponse(request.ID(), err)
}

// String to string request
func (request *request) String() string {
	return string(request.Bytes())
}

// NewRequestResponderWithState new request reponser with state
func NewRequestResponderWithState(version string, id interface{}, method string,
	params interface{}, state State) RequestResponder {
	return &request{
		RequestVersion: version,
		RequestID:      id,
		RequestMethod:  method,
		RequestParams:  params,
		requestState:   state,
	}
}

// NewRequestResponder responder
func NewRequestResponder(version string, id interface{}, method string,
	params interface{}) RequestResponder {
	return NewRequestResponderWithState(version, id, method, params, State{})
}

// GenerateRequestID generate a request id
func GenerateRequestID() string {
	hash := md5.Sum([]byte(strconv.Itoa(rand.Int())))
	return hex.EncodeToString(hash[:])
}

// The bytes representation of a request will be the JSON encoded value. This
// JSON is expected to be a perfectly valid JSON-RPC request.
func (request *request) Bytes() []byte {
	b, err := json.Marshal(request)
	if err != nil {
		return nil
	}
	return b
}

func newRequestResponderFromJSON(jsonRequest []byte, isPartOfBatch bool,
	state State) (RequestResponder, interface{}, int, string) {
	var requestMap map[string]interface{}
	err := json.Unmarshal(jsonRequest, &requestMap)
	if err != nil {
		errCode := ParseError

		if isPartOfBatch {
			errCode = InvalidRequest
		}

		// It is unlikely that we will have an "id" but we might as well try.
		return nil, requestMap["id"], errCode, ErrorMessageForCode(errCode)
	}

	// Catch some type errors before creating the real request.
	if _, ok := requestMap["jsonrpc"].(string); !ok {
		return nil, requestMap["id"],
			InvalidRequest, "Version (jsonrpc) must be a string."
	}
	if _, ok := requestMap["method"].(string); !ok {
		return nil, requestMap["id"], InvalidRequest, "Method must be a string."
	}

	return NewRequestResponderWithState(
		requestMap["jsonrpc"].(string),
		requestMap["id"],
		requestMap["method"].(string),
		requestMap["params"],
		state,
	), requestMap["id"], Success, ""
}

// NewRequestFromJSON request from json
func NewRequestFromJSON(data []byte) (RequestResponder, error) {
	if len(data) == 0 {
		return nil, errors.New("Empty input")
	}

	r, _, _, errMessage := newRequestResponderFromJSON(data, false, nil)
	if errMessage != "" {
		return nil, errors.New(errMessage)
	}

	return r, nil
}

// NewRequestsFromJSON multiply requests from json
func NewRequestsFromJSON(data []byte) ([]RequestResponder, error) {
	if len(data) == 0 {
		return nil, errors.New("Empty input")
	}

	// Single request
	if data[0] != '[' {
		request, err := NewRequestFromJSON(data)
		if err != nil {
			return nil, err
		}

		return []RequestResponder{request}, err
	}

	// Multi request.
	rawRequests := []*request{}
	err := json.Unmarshal(data, &rawRequests)
	if err != nil {
		return nil, err
	}

	requests := make([]RequestResponder, len(rawRequests))
	for i := range rawRequests {
		requests[i] = rawRequests[i]
	}

	return requests, err
}
