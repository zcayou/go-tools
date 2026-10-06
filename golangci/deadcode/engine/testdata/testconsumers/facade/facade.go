// Package facade renames part of api for consumers.
package facade

import "testconsumers/api"

// Handler is api.HandlerFunc under a second name.
type Handler = api.HandlerFunc
