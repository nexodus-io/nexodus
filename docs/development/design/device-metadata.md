# Device Metadata

> [Issue #1171](https://github.com/nexodus-io/nexodus/issues/1171)

## Summary

The [nexlink design proposal](https://docs.nexodus.io/development/design/nexlink/#nexodus-api-changes-required) described a use case for an external application that would like to store custom metadata about a device. This proposal describes a mechanism for storing such metadata.

## Proposal

Instead of changing the Device table, we will create a new table called `device_metadata`. I propose keeping it separate from the Device table while we work through how we want this to work. This table will have the following columns:

```go
type DeviceMetadata struct {
    DeviceID  uuid.UUID      `json:"device_id" gorm:"type:uuid;primary_key"`
    Key       string         `json:"key"       gorm:"primary_key"`
    Value     interface{}    `json:"value"     gorm:"type:JSONB; serializer:json"`
    Revision  uint64         `json:"revision"  gorm:"type:bigserial;index:"`
    DeletedAt gorm.DeletedAt `json:"-"         gorm:"index"`
    CreatedAt time.Time      `json:"-"`
    UpdatedAt time.Time      `json:"-"`
}
```

`DeviceMetadataInstance` to `Device` has an N:1 ratio.

`key` must be a string. `value` is `json` that will be serialized and stored as a JSONB. Each `key` must be unique for a given `device_id`. Metadata is stored by `key` to make it easy to create, update, and delete on a per-key basis.

The proposed API additions are as follows:

```go
        // Get all metadata for all devices in an organization
        private.GET("/organizations/:organization/metadata", api.ListOrganizationMetadata)
        // Get all metadata for a device
        private.GET("/devices/:id/metadata", api.GetDeviceMetadata)
        // Get a specific metadata key for a device
        private.GET("/devices/:id/metadata/:key", api.GetDeviceMetadataKey)
        // Set a specific metadata key for a device
        private.PATCH("/devices/:id/metadata/:key", api.UpdateDeviceMetadataKey)
        // Delete all metadata for a device
        private.DELETE("/devices/:id/metadata", api.DeleteDeviceMetadata)
        // Delete a specific metadata key for a device
        private.DELETE("/devices/:id/metadata/:key", api.DeleteDeviceMetadataKey)
```

## Alternatives Considered

- The `nexlink` proposal could have suggested expanding Nexodus with the metadata needed, but it seemed better to focus on how to build this as a layered application.
- Metadata could have been added directly to the `Device` table, but that seemed more invasive than necessary, at least for now.

## References

- <https://docs.nexodus.io/development/design/nexlink/#nexodus-api-changes-required>