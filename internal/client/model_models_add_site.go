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

// checks if the ModelsAddSite type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ModelsAddSite{}

// ModelsAddSite struct for ModelsAddSite
type ModelsAddSite struct {
	Name             *string `json:"name,omitempty"`
	Platform         *string `json:"platform,omitempty"`
	PublicKey        *string `json:"public_key,omitempty"`
	ServiceNetworkId *string `json:"service_network_id,omitempty"`
}

// NewModelsAddSite instantiates a new ModelsAddSite object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewModelsAddSite() *ModelsAddSite {
	this := ModelsAddSite{}
	return &this
}

// NewModelsAddSiteWithDefaults instantiates a new ModelsAddSite object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewModelsAddSiteWithDefaults() *ModelsAddSite {
	this := ModelsAddSite{}
	return &this
}

// GetName returns the Name field value if set, zero value otherwise.
func (o *ModelsAddSite) GetName() string {
	if o == nil || IsNil(o.Name) {
		var ret string
		return ret
	}
	return *o.Name
}

// GetNameOk returns a tuple with the Name field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddSite) GetNameOk() (*string, bool) {
	if o == nil || IsNil(o.Name) {
		return nil, false
	}
	return o.Name, true
}

// HasName returns a boolean if a field has been set.
func (o *ModelsAddSite) HasName() bool {
	if o != nil && !IsNil(o.Name) {
		return true
	}

	return false
}

// SetName gets a reference to the given string and assigns it to the Name field.
func (o *ModelsAddSite) SetName(v string) {
	o.Name = &v
}

// GetPlatform returns the Platform field value if set, zero value otherwise.
func (o *ModelsAddSite) GetPlatform() string {
	if o == nil || IsNil(o.Platform) {
		var ret string
		return ret
	}
	return *o.Platform
}

// GetPlatformOk returns a tuple with the Platform field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddSite) GetPlatformOk() (*string, bool) {
	if o == nil || IsNil(o.Platform) {
		return nil, false
	}
	return o.Platform, true
}

// HasPlatform returns a boolean if a field has been set.
func (o *ModelsAddSite) HasPlatform() bool {
	if o != nil && !IsNil(o.Platform) {
		return true
	}

	return false
}

// SetPlatform gets a reference to the given string and assigns it to the Platform field.
func (o *ModelsAddSite) SetPlatform(v string) {
	o.Platform = &v
}

// GetPublicKey returns the PublicKey field value if set, zero value otherwise.
func (o *ModelsAddSite) GetPublicKey() string {
	if o == nil || IsNil(o.PublicKey) {
		var ret string
		return ret
	}
	return *o.PublicKey
}

// GetPublicKeyOk returns a tuple with the PublicKey field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddSite) GetPublicKeyOk() (*string, bool) {
	if o == nil || IsNil(o.PublicKey) {
		return nil, false
	}
	return o.PublicKey, true
}

// HasPublicKey returns a boolean if a field has been set.
func (o *ModelsAddSite) HasPublicKey() bool {
	if o != nil && !IsNil(o.PublicKey) {
		return true
	}

	return false
}

// SetPublicKey gets a reference to the given string and assigns it to the PublicKey field.
func (o *ModelsAddSite) SetPublicKey(v string) {
	o.PublicKey = &v
}

// GetServiceNetworkId returns the ServiceNetworkId field value if set, zero value otherwise.
func (o *ModelsAddSite) GetServiceNetworkId() string {
	if o == nil || IsNil(o.ServiceNetworkId) {
		var ret string
		return ret
	}
	return *o.ServiceNetworkId
}

// GetServiceNetworkIdOk returns a tuple with the ServiceNetworkId field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddSite) GetServiceNetworkIdOk() (*string, bool) {
	if o == nil || IsNil(o.ServiceNetworkId) {
		return nil, false
	}
	return o.ServiceNetworkId, true
}

// HasServiceNetworkId returns a boolean if a field has been set.
func (o *ModelsAddSite) HasServiceNetworkId() bool {
	if o != nil && !IsNil(o.ServiceNetworkId) {
		return true
	}

	return false
}

// SetServiceNetworkId gets a reference to the given string and assigns it to the ServiceNetworkId field.
func (o *ModelsAddSite) SetServiceNetworkId(v string) {
	o.ServiceNetworkId = &v
}

func (o ModelsAddSite) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ModelsAddSite) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Name) {
		toSerialize["name"] = o.Name
	}
	if !IsNil(o.Platform) {
		toSerialize["platform"] = o.Platform
	}
	if !IsNil(o.PublicKey) {
		toSerialize["public_key"] = o.PublicKey
	}
	if !IsNil(o.ServiceNetworkId) {
		toSerialize["service_network_id"] = o.ServiceNetworkId
	}
	return toSerialize, nil
}

type NullableModelsAddSite struct {
	value *ModelsAddSite
	isSet bool
}

func (v NullableModelsAddSite) Get() *ModelsAddSite {
	return v.value
}

func (v *NullableModelsAddSite) Set(val *ModelsAddSite) {
	v.value = val
	v.isSet = true
}

func (v NullableModelsAddSite) IsSet() bool {
	return v.isSet
}

func (v *NullableModelsAddSite) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableModelsAddSite(val *ModelsAddSite) *NullableModelsAddSite {
	return &NullableModelsAddSite{value: val, isSet: true}
}

func (v NullableModelsAddSite) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableModelsAddSite) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
