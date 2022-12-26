// Code generated by go-swagger; DO NOT EDIT.

// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package runtime

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"net/http"

	"github.com/go-openapi/runtime"
)

// GetRuntimeStartupOKCode is the HTTP code returned for type GetRuntimeStartupOK
const GetRuntimeStartupOKCode int = 200

/*
GetRuntimeStartupOK Success

swagger:response getRuntimeStartupOK
*/
type GetRuntimeStartupOK struct {
}

// NewGetRuntimeStartupOK creates GetRuntimeStartupOK with default headers values
func NewGetRuntimeStartupOK() *GetRuntimeStartupOK {

	return &GetRuntimeStartupOK{}
}

// WriteResponse to the client
func (o *GetRuntimeStartupOK) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.Header().Del(runtime.HeaderContentType) //Remove Content-Type on empty responses

	rw.WriteHeader(200)
}

// GetRuntimeStartupInternalServerErrorCode is the HTTP code returned for type GetRuntimeStartupInternalServerError
const GetRuntimeStartupInternalServerErrorCode int = 500

/*
GetRuntimeStartupInternalServerError Failed

swagger:response getRuntimeStartupInternalServerError
*/
type GetRuntimeStartupInternalServerError struct {
}

// NewGetRuntimeStartupInternalServerError creates GetRuntimeStartupInternalServerError with default headers values
func NewGetRuntimeStartupInternalServerError() *GetRuntimeStartupInternalServerError {

	return &GetRuntimeStartupInternalServerError{}
}

// WriteResponse to the client
func (o *GetRuntimeStartupInternalServerError) WriteResponse(rw http.ResponseWriter, producer runtime.Producer) {

	rw.Header().Del(runtime.HeaderContentType) //Remove Content-Type on empty responses

	rw.WriteHeader(500)
}