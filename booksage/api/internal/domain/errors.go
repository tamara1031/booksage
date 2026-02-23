package domain

import "errors"

var ErrNotFound = errors.New("record not found")
var ErrConcurrentUpdate = errors.New("concurrent update detected: version mismatch")
