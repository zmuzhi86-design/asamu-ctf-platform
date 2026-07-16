# Legacy migration directory

This directory is retained for historical compatibility only. The executable
migration source is `internal/platform/database/migrations`, embedded by Goose.
Do not add new migrations here; new migrations must use the numbered embedded
sequence and must be expand-and-contract safe.
