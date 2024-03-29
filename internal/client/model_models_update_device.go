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

// checks if the ModelsUpdateDevice type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ModelsUpdateDevice{}

// ModelsUpdateDevice struct for ModelsUpdateDevice
type ModelsUpdateDevice struct {
	AdvertiseCidrs  []string         `json:"advertise_cidrs,omitempty"`
	Endpoints       []ModelsEndpoint `json:"endpoints,omitempty"`
	Hostname        *string          `json:"hostname,omitempty"`
	Relay           *bool            `json:"relay,omitempty"`
	Revision        *int32           `json:"revision,omitempty"`
	SecurityGroupId *string          `json:"security_group_id,omitempty"`
	SymmetricNat    *bool            `json:"symmetric_nat,omitempty"`
	VpcId           *string          `json:"vpc_id,omitempty"`
}

// NewModelsUpdateDevice instantiates a new ModelsUpdateDevice object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewModelsUpdateDevice() *ModelsUpdateDevice {
	this := ModelsUpdateDevice{}
	return &this
}

// NewModelsUpdateDeviceWithDefaults instantiates a new ModelsUpdateDevice object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewModelsUpdateDeviceWithDefaults() *ModelsUpdateDevice {
	this := ModelsUpdateDevice{}
	return &this
}

// GetAdvertiseCidrs returns the AdvertiseCidrs field value if set, zero value otherwise.
func (o *ModelsUpdateDevice) GetAdvertiseCidrs() []string {
	if o == nil || IsNil(o.AdvertiseCidrs) {
		var ret []string
		return ret
	}
	return o.AdvertiseCidrs
}

// GetAdvertiseCidrsOk returns a tuple with the AdvertiseCidrs field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsUpdateDevice) GetAdvertiseCidrsOk() ([]string, bool) {
	if o == nil || IsNil(o.AdvertiseCidrs) {
		return nil, false
	}
	return o.AdvertiseCidrs, true
}

// HasAdvertiseCidrs returns a boolean if a field has been set.
func (o *ModelsUpdateDevice) HasAdvertiseCidrs() bool {
	if o != nil && !IsNil(o.AdvertiseCidrs) {
		return true
	}

	return false
}

// SetAdvertiseCidrs gets a reference to the given []string and assigns it to the AdvertiseCidrs field.
func (o *ModelsUpdateDevice) SetAdvertiseCidrs(v []string) {
	o.AdvertiseCidrs = v
}

// GetEndpoints returns the Endpoints field value if set, zero value otherwise.
func (o *ModelsUpdateDevice) GetEndpoints() []ModelsEndpoint {
	if o == nil || IsNil(o.Endpoints) {
		var ret []ModelsEndpoint
		return ret
	}
	return o.Endpoints
}

// GetEndpointsOk returns a tuple with the Endpoints field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsUpdateDevice) GetEndpointsOk() ([]ModelsEndpoint, bool) {
	if o == nil || IsNil(o.Endpoints) {
		return nil, false
	}
	return o.Endpoints, true
}

// HasEndpoints returns a boolean if a field has been set.
func (o *ModelsUpdateDevice) HasEndpoints() bool {
	if o != nil && !IsNil(o.Endpoints) {
		return true
	}

	return false
}

// SetEndpoints gets a reference to the given []ModelsEndpoint and assigns it to the Endpoints field.
func (o *ModelsUpdateDevice) SetEndpoints(v []ModelsEndpoint) {
	o.Endpoints = v
}

// GetHostname returns the Hostname field value if set, zero value otherwise.
func (o *ModelsUpdateDevice) GetHostname() string {
	if o == nil || IsNil(o.Hostname) {
		var ret string
		return ret
	}
	return *o.Hostname
}

// GetHostnameOk returns a tuple with the Hostname field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsUpdateDevice) GetHostnameOk() (*string, bool) {
	if o == nil || IsNil(o.Hostname) {
		return nil, false
	}
	return o.Hostname, true
}

// HasHostname returns a boolean if a field has been set.
func (o *ModelsUpdateDevice) HasHostname() bool {
	if o != nil && !IsNil(o.Hostname) {
		return true
	}

	return false
}

// SetHostname gets a reference to the given string and assigns it to the Hostname field.
func (o *ModelsUpdateDevice) SetHostname(v string) {
	o.Hostname = &v
}

// GetRelay returns the Relay field value if set, zero value otherwise.
func (o *ModelsUpdateDevice) GetRelay() bool {
	if o == nil || IsNil(o.Relay) {
		var ret bool
		return ret
	}
	return *o.Relay
}

// GetRelayOk returns a tuple with the Relay field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsUpdateDevice) GetRelayOk() (*bool, bool) {
	if o == nil || IsNil(o.Relay) {
		return nil, false
	}
	return o.Relay, true
}

// HasRelay returns a boolean if a field has been set.
func (o *ModelsUpdateDevice) HasRelay() bool {
	if o != nil && !IsNil(o.Relay) {
		return true
	}

	return false
}

// SetRelay gets a reference to the given bool and assigns it to the Relay field.
func (o *ModelsUpdateDevice) SetRelay(v bool) {
	o.Relay = &v
}

// GetRevision returns the Revision field value if set, zero value otherwise.
func (o *ModelsUpdateDevice) GetRevision() int32 {
	if o == nil || IsNil(o.Revision) {
		var ret int32
		return ret
	}
	return *o.Revision
}

// GetRevisionOk returns a tuple with the Revision field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsUpdateDevice) GetRevisionOk() (*int32, bool) {
	if o == nil || IsNil(o.Revision) {
		return nil, false
	}
	return o.Revision, true
}

// HasRevision returns a boolean if a field has been set.
func (o *ModelsUpdateDevice) HasRevision() bool {
	if o != nil && !IsNil(o.Revision) {
		return true
	}

	return false
}

// SetRevision gets a reference to the given int32 and assigns it to the Revision field.
func (o *ModelsUpdateDevice) SetRevision(v int32) {
	o.Revision = &v
}

// GetSecurityGroupId returns the SecurityGroupId field value if set, zero value otherwise.
func (o *ModelsUpdateDevice) GetSecurityGroupId() string {
	if o == nil || IsNil(o.SecurityGroupId) {
		var ret string
		return ret
	}
	return *o.SecurityGroupId
}

// GetSecurityGroupIdOk returns a tuple with the SecurityGroupId field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsUpdateDevice) GetSecurityGroupIdOk() (*string, bool) {
	if o == nil || IsNil(o.SecurityGroupId) {
		return nil, false
	}
	return o.SecurityGroupId, true
}

// HasSecurityGroupId returns a boolean if a field has been set.
func (o *ModelsUpdateDevice) HasSecurityGroupId() bool {
	if o != nil && !IsNil(o.SecurityGroupId) {
		return true
	}

	return false
}

// SetSecurityGroupId gets a reference to the given string and assigns it to the SecurityGroupId field.
func (o *ModelsUpdateDevice) SetSecurityGroupId(v string) {
	o.SecurityGroupId = &v
}

// GetSymmetricNat returns the SymmetricNat field value if set, zero value otherwise.
func (o *ModelsUpdateDevice) GetSymmetricNat() bool {
	if o == nil || IsNil(o.SymmetricNat) {
		var ret bool
		return ret
	}
	return *o.SymmetricNat
}

// GetSymmetricNatOk returns a tuple with the SymmetricNat field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsUpdateDevice) GetSymmetricNatOk() (*bool, bool) {
	if o == nil || IsNil(o.SymmetricNat) {
		return nil, false
	}
	return o.SymmetricNat, true
}

// HasSymmetricNat returns a boolean if a field has been set.
func (o *ModelsUpdateDevice) HasSymmetricNat() bool {
	if o != nil && !IsNil(o.SymmetricNat) {
		return true
	}

	return false
}

// SetSymmetricNat gets a reference to the given bool and assigns it to the SymmetricNat field.
func (o *ModelsUpdateDevice) SetSymmetricNat(v bool) {
	o.SymmetricNat = &v
}

// GetVpcId returns the VpcId field value if set, zero value otherwise.
func (o *ModelsUpdateDevice) GetVpcId() string {
	if o == nil || IsNil(o.VpcId) {
		var ret string
		return ret
	}
	return *o.VpcId
}

// GetVpcIdOk returns a tuple with the VpcId field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsUpdateDevice) GetVpcIdOk() (*string, bool) {
	if o == nil || IsNil(o.VpcId) {
		return nil, false
	}
	return o.VpcId, true
}

// HasVpcId returns a boolean if a field has been set.
func (o *ModelsUpdateDevice) HasVpcId() bool {
	if o != nil && !IsNil(o.VpcId) {
		return true
	}

	return false
}

// SetVpcId gets a reference to the given string and assigns it to the VpcId field.
func (o *ModelsUpdateDevice) SetVpcId(v string) {
	o.VpcId = &v
}

func (o ModelsUpdateDevice) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ModelsUpdateDevice) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.AdvertiseCidrs) {
		toSerialize["advertise_cidrs"] = o.AdvertiseCidrs
	}
	if !IsNil(o.Endpoints) {
		toSerialize["endpoints"] = o.Endpoints
	}
	if !IsNil(o.Hostname) {
		toSerialize["hostname"] = o.Hostname
	}
	if !IsNil(o.Relay) {
		toSerialize["relay"] = o.Relay
	}
	if !IsNil(o.Revision) {
		toSerialize["revision"] = o.Revision
	}
	if !IsNil(o.SecurityGroupId) {
		toSerialize["security_group_id"] = o.SecurityGroupId
	}
	if !IsNil(o.SymmetricNat) {
		toSerialize["symmetric_nat"] = o.SymmetricNat
	}
	if !IsNil(o.VpcId) {
		toSerialize["vpc_id"] = o.VpcId
	}
	return toSerialize, nil
}

type NullableModelsUpdateDevice struct {
	value *ModelsUpdateDevice
	isSet bool
}

func (v NullableModelsUpdateDevice) Get() *ModelsUpdateDevice {
	return v.value
}

func (v *NullableModelsUpdateDevice) Set(val *ModelsUpdateDevice) {
	v.value = val
	v.isSet = true
}

func (v NullableModelsUpdateDevice) IsSet() bool {
	return v.isSet
}

func (v *NullableModelsUpdateDevice) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableModelsUpdateDevice(val *ModelsUpdateDevice) *NullableModelsUpdateDevice {
	return &NullableModelsUpdateDevice{value: val, isSet: true}
}

func (v NullableModelsUpdateDevice) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableModelsUpdateDevice) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
