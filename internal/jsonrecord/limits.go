// Package jsonrecord defines bounds shared by local JSON record readers.
// Capture-hook payloads retain their own separate admission contract.
package jsonrecord

// MaxDepth bounds value nesting with the root at depth zero.
const MaxDepth = 64
