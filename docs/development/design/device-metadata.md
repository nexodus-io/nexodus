# Device Metadata

> [Issue #1171](https://github.com/nexodus-io/nexodus/issues/1171)

## Summary

The [nexlink design proposal](https://docs.nexodus.io/development/design/nexlink/#nexodus-api-changes-required) described a use case for an external application that would like to store custom metadata about a device. This proposal describes a mechanism for storing such metadata.

## Proposal

Instead of changing the Device table, we will create a new table called DeviceMetadata. I propose keeping it separate from the Device table while we work through how we want this to work. This table will have the following columns:

```go
type Base struct {
    ID        uuid.UUID      `gorm:"type:uuid;primary_key;" json:"id" example:"aa22666c-0f57-45cb-a449-16efecc04f2e"`
    CreatedAt time.Time      `json:"-"`
    UpdatedAt time.Time      `json:"-"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type DeviceMetadataInstance struct {
    Base
    DeviceID uuid.UUID `json:"device_id"`
    Key      string    `json:"key"`
    Value    string    `json:"value"`
}
```

`DeviceMetadataInstance` to `Device` has an N:1 ratio.

`key` must be a string. `value` is `json` that will be serialized and stored as a string. Each `key` must be unique for a given `device_id`. Metadata is stored by `key` to make it easy to create, update, and delete on a per-key basis.

The proposed API additions are as follows:

```go
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