/*
Nexodus API

This is the Nexodus API Server.

API version: 1.0
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package client

import (
	"encoding/json"
)

// checks if the ModelsConflictsError type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ModelsConflictsError{}

// ModelsConflictsError struct for ModelsConflictsError
type ModelsConflictsError struct {
	Error *string `json:"error,omitempty"`
	Id    *string `json:"id,omitempty"`
}

// NewModelsConflictsError instantiates a new ModelsConflictsError object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewModelsConflictsError() *ModelsConflictsError {
	this := ModelsConflictsError{}
	return &this
}

// NewModelsConflictsErrorWithDefaults instantiates a new ModelsConflictsError object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewModelsConflictsErrorWithDefaults() *ModelsConflictsError {
	this := ModelsConflictsError{}
	return &this
}

// GetError returns the Error field value if set, zero value otherwise.
func (o *ModelsConflictsError) GetError() string {
	if o == nil || IsNil(o.Error) {
		var ret string
		return ret
	}
	return *o.Error
}

// GetErrorOk returns a tuple with the Error field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsConflictsError) GetErrorOk() (*string, bool) {
	if o == nil || IsNil(o.Error) {
		return nil, false
	}
	return o.Error, true
}

// HasError returns a boolean if a field has been set.
func (o *ModelsConflictsError) HasError() bool {
	if o != nil && !IsNil(o.Error) {
		return true
	}

	return false
}

// SetError gets a reference to the given string and assigns it to the Error field.
func (o *ModelsConflictsError) SetError(v string) {
	o.Error = &v
}

// GetId returns the Id field value if set, zero value otherwise.
func (o *ModelsConflictsError) GetId() string {
	if o == nil || IsNil(o.Id) {
		var ret string
		return ret
	}
	return *o.Id
}

// GetIdOk returns a tuple with the Id field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsConflictsError) GetIdOk() (*string, bool) {
	if o == nil || IsNil(o.Id) {
		return nil, false
	}
	return o.Id, true
}

// HasId returns a boolean if a field has been set.
func (o *ModelsConflictsError) HasId() bool {
	if o != nil && !IsNil(o.Id) {
		return true
	}

	return false
}

// SetId gets a reference to the given string and assigns it to the Id field.
func (o *ModelsConflictsError) SetId(v string) {
	o.Id = &v
}

func (o ModelsConflictsError) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ModelsConflictsError) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Error) {
		toSerialize["error"] = o.Error
	}
	if !IsNil(o.Id) {
		toSerialize["id"] = o.Id
	}
	return toSerialize, nil
}

type NullableModelsConflictsError struct {
	value *ModelsConflictsError
	isSet bool
}

func (v NullableModelsConflictsError) Get() *ModelsConflictsError {
	return v.value
}

func (v *NullableModelsConflictsError) Set(val *ModelsConflictsError) {
	v.value = val
	v.isSet = true
}

func (v NullableModelsConflictsError) IsSet() bool {
	return v.isSet
}

func (v *NullableModelsConflictsError) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableModelsConflictsError(val *ModelsConflictsError) *NullableModelsConflictsError {
	return &NullableModelsConflictsError{value: val, isSet: true}
}

func (v NullableModelsConflictsError) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableModelsConflictsError) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
