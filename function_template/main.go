package template

import (
	"context"
	"fmt"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/cloudevents/sdk-go/v2/event"
)

func init() {
	// Register a CloudEvent function with the Functions Framework
	functions.CloudEvent("template", template)
}

// Function template accepts and handles a CloudEvent object
func template(ctx context.Context, e event.Event) error {
	fmt.Println(e)

	// Return nil if no error occurred
	return nil
}
