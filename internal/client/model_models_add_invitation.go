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

// checks if the ModelsAddInvitation type satisfies the MappedNullable interface at compile time
var _ MappedNullable = &ModelsAddInvitation{}

// ModelsAddInvitation struct for ModelsAddInvitation
type ModelsAddInvitation struct {
	// The email address of the user to invite (one of email or user_id is required)
	Email          *string  `json:"email,omitempty"`
	OrganizationId *string  `json:"organization_id,omitempty"`
	Roles          []string `json:"roles,omitempty"`
	// The user id to invite (one of email or user_id is required)
	UserId *string `json:"user_id,omitempty"`
}

// NewModelsAddInvitation instantiates a new ModelsAddInvitation object
// This constructor will assign default values to properties that have it defined,
// and makes sure properties required by API are set, but the set of arguments
// will change when the set of required properties is changed
func NewModelsAddInvitation() *ModelsAddInvitation {
	this := ModelsAddInvitation{}
	return &this
}

// NewModelsAddInvitationWithDefaults instantiates a new ModelsAddInvitation object
// This constructor will only assign default values to properties that have it defined,
// but it doesn't guarantee that properties required by API are set
func NewModelsAddInvitationWithDefaults() *ModelsAddInvitation {
	this := ModelsAddInvitation{}
	return &this
}

// GetEmail returns the Email field value if set, zero value otherwise.
func (o *ModelsAddInvitation) GetEmail() string {
	if o == nil || IsNil(o.Email) {
		var ret string
		return ret
	}
	return *o.Email
}

// GetEmailOk returns a tuple with the Email field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddInvitation) GetEmailOk() (*string, bool) {
	if o == nil || IsNil(o.Email) {
		return nil, false
	}
	return o.Email, true
}

// HasEmail returns a boolean if a field has been set.
func (o *ModelsAddInvitation) HasEmail() bool {
	if o != nil && !IsNil(o.Email) {
		return true
	}

	return false
}

// SetEmail gets a reference to the given string and assigns it to the Email field.
func (o *ModelsAddInvitation) SetEmail(v string) {
	o.Email = &v
}

// GetOrganizationId returns the OrganizationId field value if set, zero value otherwise.
func (o *ModelsAddInvitation) GetOrganizationId() string {
	if o == nil || IsNil(o.OrganizationId) {
		var ret string
		return ret
	}
	return *o.OrganizationId
}

// GetOrganizationIdOk returns a tuple with the OrganizationId field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddInvitation) GetOrganizationIdOk() (*string, bool) {
	if o == nil || IsNil(o.OrganizationId) {
		return nil, false
	}
	return o.OrganizationId, true
}

// HasOrganizationId returns a boolean if a field has been set.
func (o *ModelsAddInvitation) HasOrganizationId() bool {
	if o != nil && !IsNil(o.OrganizationId) {
		return true
	}

	return false
}

// SetOrganizationId gets a reference to the given string and assigns it to the OrganizationId field.
func (o *ModelsAddInvitation) SetOrganizationId(v string) {
	o.OrganizationId = &v
}

// GetRoles returns the Roles field value if set, zero value otherwise.
func (o *ModelsAddInvitation) GetRoles() []string {
	if o == nil || IsNil(o.Roles) {
		var ret []string
		return ret
	}
	return o.Roles
}

// GetRolesOk returns a tuple with the Roles field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddInvitation) GetRolesOk() ([]string, bool) {
	if o == nil || IsNil(o.Roles) {
		return nil, false
	}
	return o.Roles, true
}

// HasRoles returns a boolean if a field has been set.
func (o *ModelsAddInvitation) HasRoles() bool {
	if o != nil && !IsNil(o.Roles) {
		return true
	}

	return false
}

// SetRoles gets a reference to the given []string and assigns it to the Roles field.
func (o *ModelsAddInvitation) SetRoles(v []string) {
	o.Roles = v
}

// GetUserId returns the UserId field value if set, zero value otherwise.
func (o *ModelsAddInvitation) GetUserId() string {
	if o == nil || IsNil(o.UserId) {
		var ret string
		return ret
	}
	return *o.UserId
}

// GetUserIdOk returns a tuple with the UserId field value if set, nil otherwise
// and a boolean to check if the value has been set.
func (o *ModelsAddInvitation) GetUserIdOk() (*string, bool) {
	if o == nil || IsNil(o.UserId) {
		return nil, false
	}
	return o.UserId, true
}

// HasUserId returns a boolean if a field has been set.
func (o *ModelsAddInvitation) HasUserId() bool {
	if o != nil && !IsNil(o.UserId) {
		return true
	}

	return false
}

// SetUserId gets a reference to the given string and assigns it to the UserId field.
func (o *ModelsAddInvitation) SetUserId(v string) {
	o.UserId = &v
}

func (o ModelsAddInvitation) MarshalJSON() ([]byte, error) {
	toSerialize, err := o.ToMap()
	if err != nil {
		return []byte{}, err
	}
	return json.Marshal(toSerialize)
}

func (o ModelsAddInvitation) ToMap() (map[string]interface{}, error) {
	toSerialize := map[string]interface{}{}
	if !IsNil(o.Email) {
		toSerialize["email"] = o.Email
	}
	if !IsNil(o.OrganizationId) {
		toSerialize["organization_id"] = o.OrganizationId
	}
	if !IsNil(o.Roles) {
		toSerialize["roles"] = o.Roles
	}
	if !IsNil(o.UserId) {
		toSerialize["user_id"] = o.UserId
	}
	return toSerialize, nil
}

type NullableModelsAddInvitation struct {
	value *ModelsAddInvitation
	isSet bool
}

func (v NullableModelsAddInvitation) Get() *ModelsAddInvitation {
	return v.value
}

func (v *NullableModelsAddInvitation) Set(val *ModelsAddInvitation) {
	v.value = val
	v.isSet = true
}

func (v NullableModelsAddInvitation) IsSet() bool {
	return v.isSet
}

func (v *NullableModelsAddInvitation) Unset() {
	v.value = nil
	v.isSet = false
}

func NewNullableModelsAddInvitation(val *ModelsAddInvitation) *NullableModelsAddInvitation {
	return &NullableModelsAddInvitation{value: val, isSet: true}
}

func (v NullableModelsAddInvitation) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.value)
}

func (v *NullableModelsAddInvitation) UnmarshalJSON(src []byte) error {
	v.isSet = true
	return json.Unmarshal(src, &v.value)
}
