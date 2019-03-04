package jet

var abortTemplateOnError = true

// SetAbortTemplateOnError controls whether the template rendering process should be aborted when an error is encountered.
/// Default behavior is to abort the template rendering process when an error is encountered, so abortOnError == true.
func SetAbortTemplateOnError(abortOnError bool) {
	abortTemplateOnError = abortOnError
}
