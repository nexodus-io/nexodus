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

// checks if the ModelsAddDevice type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ModelsAddDevice{}

// ModelsAddDevice struct for ModelsAddDevice
type ModelsAddDevice struct {
	AdvertiseCidrs  []string         `json:"advertise_cidrs,omitempty"`
	Endpoints       []ModelsEndpoint `json:"endpoints,omitempty"`
	Hostname        *string          `json:"hostname,omitempty"`
	Ipv4TunnelIps   []ModelsTunnelIP `json:"ipv4_tunnel_ips,omitempty"`
	Os              *string          `json:"os,omitempty"`
	PublicKey       *string          `json:"public_key,omitempty"`
	Relay           *bool            `json:"relay,omitempty"`
	SecurityGroupId *string          `json:"security_group_id,omitempty"`
	SymmetricNat    *bool            `json:"symmetric_nat,omitempty"`
	VpcId           *string          `json:"vpc_id,omitempty"`
}

// NewModelsAddDevice instantiates a new ModelsAddDevice object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewModelsAddDevice() *ModelsAddDevice {
	this := ModelsAddDevice{}
	return &this
}

// NewModelsAddDeviceWithDefaults instantiates a new ModelsAddDevice object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewModelsAddDeviceWithDefaults() *ModelsAddDevice {
	this := ModelsAddDevice{}
	return &this
}

// GetAdvertiseCidrs returns the AdvertiseCidrs field value if set, zero value otherwise.
func (o *ModelsAddDevice) GetAdvertiseCidrs() []string {
	if o == nil || IsNil(o.AdvertiseCidrs) {
		var ret []string
		return ret
	}
	return o.AdvertiseCidrs
}

// GetAdvertiseCidrsOk returns a tuple with the AdvertiseCidrs field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddDevice) GetAdvertiseCidrsOk() ([]string, bool) {
	if o == nil || IsNil(o.AdvertiseCidrs) {
		return nil, false
	}
	return o.AdvertiseCidrs, true
}

// HasAdvertiseCidrs returns a boolean if a field has been set.
func (o *ModelsAddDevice) HasAdvertiseCidrs() bool {
	if o != nil && !IsNil(o.AdvertiseCidrs) {
		return true
	}

	return false
}

// SetAdvertiseCidrs gets a reference to the given []string and assigns it to the AdvertiseCidrs field.
func (o *ModelsAddDevice) SetAdvertiseCidrs(v []string) {
	o.AdvertiseCidrs = v
}

// GetEndpoints returns the Endpoints field value if set, zero value otherwise.
func (o *ModelsAddDevice) GetEndpoints() []ModelsEndpoint {
	if o == nil || IsNil(o.Endpoints) {
		var ret []ModelsEndpoint
		return ret
	}
	return o.Endpoints
}

// GetEndpointsOk returns a tuple with the Endpoints field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddDevice) GetEndpointsOk() ([]ModelsEndpoint, bool) {
	if o == nil || IsNil(o.Endpoints) {
		return nil, false
	}
	return o.Endpoints, true
}

// HasEndpoints returns a boolean if a field has been set.
func (o *ModelsAddDevice) HasEndpoints() bool {
	if o != nil && !IsNil(o.Endpoints) {
		return true
	}

	return false
}

// SetEndpoints gets a reference to the given []ModelsEndpoint and assigns it to the Endpoints field.
func (o *ModelsAddDevice) SetEndpoints(v []ModelsEndpoint) {
	o.Endpoints = v
}

// GetHostname returns the Hostname field value if set, zero value otherwise.
func (o *ModelsAddDevice) GetHostname() string {
	if o == nil || IsNil(o.Hostname) {
		var ret string
		return ret
	}
	return *o.Hostname
}

// GetHostnameOk returns a tuple with the Hostname field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddDevice) GetHostnameOk() (*string, bool) {
	if o == nil || IsNil(o.Hostname) {
		return nil, false
	}
	return o.Hostname, true
}

// HasHostname returns a boolean if a field has been set.
func (o *ModelsAddDevice) HasHostname() bool {
	if o != nil && !IsNil(o.Hostname) {
		return true
	}

	return false
}

// SetHostname gets a reference to the given string and assigns it to the Hostname field.
func (o *ModelsAddDevice) SetHostname(v string) {
	o.Hostname = &v
}

// GetIpv4TunnelIps returns the Ipv4TunnelIps field value if set, zero value otherwise.
func (o *ModelsAddDevice) GetIpv4TunnelIps() []ModelsTunnelIP {
	if o == nil || IsNil(o.Ipv4TunnelIps) {
		var ret []ModelsTunnelIP
		return ret
	}
	return o.Ipv4TunnelIps
}

// GetIpv4TunnelIpsOk returns a tuple with the Ipv4TunnelIps field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddDevice) GetIpv4TunnelIpsOk() ([]ModelsTunnelIP, bool) {
	if o == nil || IsNil(o.Ipv4TunnelIps) {
		return nil, false
	}
	return o.Ipv4TunnelIps, true
}

// HasIpv4TunnelIps returns a boolean if a field has been set.
func (o *ModelsAddDevice) HasIpv4TunnelIps() bool {
	if o != nil && !IsNil(o.Ipv4TunnelIps) {
		return true
	}

	return false
}

// SetIpv4TunnelIps gets a reference to the given []ModelsTunnelIP and assigns it to the Ipv4TunnelIps field.
func (o *ModelsAddDevice) SetIpv4TunnelIps(v []ModelsTunnelIP) {
	o.Ipv4TunnelIps = v
}

// GetOs returns the Os field value if set, zero value otherwise.
func (o *ModelsAddDevice) GetOs() string {
	if o == nil || IsNil(o.Os) {
		var ret string
		return ret
	}
	return *o.Os
}

// GetOsOk returns a tuple with the Os field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddDevice) GetOsOk() (*string, bool) {
	if o == nil || IsNil(o.Os) {
		return nil, false
	}
	return o.Os, true
}

// HasOs returns a boolean if a field has been set.
func (o *ModelsAddDevice) HasOs() bool {
	if o != nil && !IsNil(o.Os) {
		return true
	}

	return false
}

// SetOs gets a reference to the given string and assigns it to the Os field.
func (o *ModelsAddDevice) SetOs(v string) {
	o.Os = &v
}

// GetPublicKey returns the PublicKey field value if set, zero value otherwise.
func (o *ModelsAddDevice) GetPublicKey() string {
	if o == nil || IsNil(o.PublicKey) {
		var ret string
		return ret
	}
	return *o.PublicKey
}

// GetPublicKeyOk returns a tuple with the PublicKey field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddDevice) GetPublicKeyOk() (*string, bool) {
	if o == nil || IsNil(o.PublicKey) {
		return nil, false
	}
	return o.PublicKey, true
}

// HasPublicKey returns a boolean if a field has been set.
func (o *ModelsAddDevice) HasPublicKey() bool {
	if o != nil && !IsNil(o.PublicKey) {
		return true
	}

	return false
}

// SetPublicKey gets a reference to the given string and assigns it to the PublicKey field.
func (o *ModelsAddDevice) SetPublicKey(v string) {
	o.PublicKey = &v
}

// GetRelay returns the Relay field value if set, zero value otherwise.
func (o *ModelsAddDevice) GetRelay() bool {
	if o == nil || IsNil(o.Relay) {
		var ret bool
		return ret
	}
	return *o.Relay
}

// GetRelayOk returns a tuple with the Relay field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddDevice) GetRelayOk() (*bool, bool) {
	if o == nil || IsNil(o.Relay) {
		return nil, false
	}
	return o.Relay, true
}

// HasRelay returns a boolean if a field has been set.
func (o *ModelsAddDevice) HasRelay() bool {
	if o != nil && !IsNil(o.Relay) {
		return true
	}

	return false
}

// SetRelay gets a reference to the given bool and assigns it to the Relay field.
func (o *ModelsAddDevice) SetRelay(v bool) {
	o.Relay = &v
}

// GetSecurityGroupId returns the SecurityGroupId field value if set, zero value otherwise.
func (o *ModelsAddDevice) GetSecurityGroupId() string {
	if o == nil || IsNil(o.SecurityGroupId) {
		var ret string
		return ret
	}
	return *o.SecurityGroupId
}

// GetSecurityGroupIdOk returns a tuple with the SecurityGroupId field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddDevice) GetSecurityGroupIdOk() (*string, bool) {
	if o == nil || IsNil(o.SecurityGroupId) {
		return nil, false
	}
	return o.SecurityGroupId, true
}

// HasSecurityGroupId returns a boolean if a field has been set.
func (o *ModelsAddDevice) HasSecurityGroupId() bool {
	if o != nil && !IsNil(o.SecurityGroupId) {
		return true
	}

	return false
}

// SetSecurityGroupId gets a reference to the given string and assigns it to the SecurityGroupId field.
func (o *ModelsAddDevice) SetSecurityGroupId(v string) {
	o.SecurityGroupId = &v
}

// GetSymmetricNat returns the SymmetricNat field value if set, zero value otherwise.
func (o *ModelsAddDevice) GetSymmetricNat() bool {
	if o == nil || IsNil(o.SymmetricNat) {
		var ret bool
		return ret
	}
	return *o.SymmetricNat
}

// GetSymmetricNatOk returns a tuple with the SymmetricNat field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddDevice) GetSymmetricNatOk() (*bool, bool) {
	if o == nil || IsNil(o.SymmetricNat) {
		return nil, false
	}
	return o.SymmetricNat, true
}

// HasSymmetricNat returns a boolean if a field has been set.
func (o *ModelsAddDevice) HasSymmetricNat() bool {
	if o != nil && !IsNil(o.SymmetricNat) {
		return true
	}

	return false
}

// SetSymmetricNat gets a reference to the given bool and assigns it to the SymmetricNat field.
func (o *ModelsAddDevice) SetSymmetricNat(v bool) {
	o.SymmetricNat = &v
}

// GetVpcId returns the VpcId field value if set, zero value otherwise.
func (o *ModelsAddDevice) GetVpcId() string {
	if o == nil || IsNil(o.VpcId) {
		var ret string
		return ret
	}
	return *o.VpcId
}

// GetVpcIdOk returns a tuple with the VpcId field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddDevice) GetVpcIdOk() (*string, bool) {
	if o == nil || IsNil(o.VpcId) {
		return nil, false
	}
	return o.VpcId, true
}

// HasVpcId returns a boolean if a field has been set.
func (o *ModelsAddDevice) HasVpcId() bool {
	if o != nil && !IsNil(o.VpcId) {
		return true
	}

	return false
}

// SetVpcId gets a reference to the given string and assigns it to the VpcId field.
func (o *ModelsAddDevice) SetVpcId(v string) {
	o.VpcId = &v
}

func (o ModelsAddDevice) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ModelsAddDevice) ToMap() (map[string]interface{}, error) {
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
	if !IsNil(o.Ipv4TunnelIps) {
		toSerialize["ipv4_tunnel_ips"] = o.Ipv4TunnelIps
	}
	if !IsNil(o.Os) {
		toSerialize["os"] = o.Os
	}
	if !IsNil(o.PublicKey) {
		toSerialize["public_key"] = o.PublicKey
	}
	if !IsNil(o.Relay) {
		toSerialize["relay"] = o.Relay
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

type NullableModelsAddDevice struct {
	value *ModelsAddDevice
	isSet bool
}

func (v NullableModelsAddDevice) Get() *ModelsAddDevice {
	return v.value
}

func (v *NullableModelsAddDevice) Set(val *ModelsAddDevice) {
	v.value = val
	v.isSet = true
}

func (v NullableModelsAddDevice) IsSet() bool {
	return v.isSet
}

func (v *NullableModelsAddDevice) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableModelsAddDevice(val *ModelsAddDevice) *NullableModelsAddDevice {
	return &NullableModelsAddDevice{value: val, isSet: true}
}

func (v NullableModelsAddDevice) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableModelsAddDevice) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
