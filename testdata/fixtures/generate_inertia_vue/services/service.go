// Package services holds application use cases. Business rules and entity
// validation live in domains. Writes that must commit together run inside
// storage.AtomicOps, with receivers and the queue bound through InTx(tx).
// Dependencies are wired explicitly in internal/runtime.
package services
