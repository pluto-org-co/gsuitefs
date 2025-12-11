package gsuitefs

import (
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/gmail/v1"
)

// API Permissions required to use all the googleutils implemented
// Notice this will also require to enable the APIs from the Cloud Console
// - Admin SDK API
// - Drive API
// - Gmail API
var Scopes = []string{
	admin.AdminDirectoryUserReadonlyScope,
	admin.AdminDirectoryDomainReadonlyScope,
	drive.DriveScope,
	gmail.GmailReadonlyScope,
}
