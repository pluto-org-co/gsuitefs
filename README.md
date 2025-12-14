## 📂 gsuitefs: Google Workspace Organization Explorer (Read-Only FUSE Filesystem)

**gsuitefs** is a read-only FUSE filesystem designed to explore the entire file structure of a Google Workspace Organization. It makes use of **Service Account credentials** and **Domain-Wide Delegation (DWD)** to impersonate an administrator and map the domains, users' personal drives, and shared drives into a local directory structure for easy access and analysis.

### Prerequisites and Setup

To successfully use `gsuitefs`, you must configure a Google Service Account with Domain-Wide Delegation.

##### 1. **Service Account JSON:**

Obtain the private key file for your service account in JSON format.

##### 2. **API Scopes:**

The following OAuth scopes must be enabled for the service account via Domain-Wide Delegation (DWD):

- `https://www.googleapis.com/auth/admin.directory.user.readonly`
- `https://www.googleapis.com/auth/admin.directory.domain.readonly`
- `https://www.googleapis.com/auth/drive` (for full drive access)
- `https://www.googleapis.com/auth/gmail.readonly`


##### 3. **Enabled APIs:**

The following APIs must be enabled from the Google Cloud Console for the project associated with your service account:

- **Admin SDK API**
- **Google Drive API**
- **Gmail API**

### Features

- [X] **Read-Only:** Safely explore your organization's file structure without the risk of accidental modification.
- [X] **FUSE Integration:** Mounts the entire Google Workspace hierarchy as a local directory on your machine.
- [X] **Comprehensive Coverage:** Maps:
- [X] Organization **Domains**.
- [X] **User Personal Drives** (Active and Trashed folders).
- [X] **Shared Drives** (Active and Trashed folders).
- [X] Allows for optional inclusion of **Shared Files**.
- [ ] Allows for optional inclusion of **Gmail** data (based on configuration).


* **Configurable:** Granular control over which parts of the organization structure are included in the mount.



### Installation

You can easily install `gsuitefs` using the Go toolchain:

```bash
go install github.com/pluto-org-co/gsuitefs/cmd/gsuitefs@latest
```

### Usage

To mount your Google Workspace Organization, use the following command:

```bash
gsuitefs mount --log-level -4 --config config.yaml ~/company
```

- `--log-level -4`: Sets the logging verbosity (e.g., to debug or trace).
- `--config config.yaml`: Specifies the path to your configuration file.
- `~/company`: The local directory where the Google Workspace filesystem will be mounted.



### Example Configuration (`config.yaml`)

The configuration file is crucial for authenticating and defining the scope of the mount.

```yaml
administrator-subject: administrator@example-domain.com # The admin email to impersonate
service-account-file: /path/to/service/account.json # Path to your service account key file
include:
    domains:
        users:
            personaldrive:
                active: true
                trashed: true
            sharedfiles: true # Optional: Include files shared with the user
            gmail: true # Optional: Include user's Gmail data
        groups: {} # Configuration for including groups (currently empty)
    shareddrives:
        active: true
        trashed: true
```

### Example Filesystem Structure

Below is an example of the directory structure created by `gsuitefs` when mounted, based on a real-world scenario. This structure illustrates how different organizational components are mapped to the local filesystem, with sensitive information generalized:

```
gsuitefs/
├── domains
│   ├── DOMAIN_A.com # Example Domain
│   │   └── users
│   │       ├── USER_1@DOMAIN_A.com # Example User
│   │       │   └── personal-drive
│   │       │       ├── active # User's Active Drive Files
│   │       │       └── trashed # User's Trashed Drive Files
│   │       └── USER_2@DOMAIN_A.com # Another Example User
│   │           └── personal-drive
│   │               ├── active
│   │               └── trashed
│   └── DOMAIN_B.com # Another Example Domain
│       └── users
│           ├── USER_3@DOMAIN_B.com # Example User
│           │   └── personal-drive
│           │       ├── active
│           │       └── trashed
│           └── USER_4@DOMAIN_B.com # Another Example User
│               └── personal-drive
│                   ├── active
│                   └── trashed
│
└── shared-drives
    ├── DRIVE_PROJECT_ACTIVITIES # Example Shared Drive
    │   ├── active
    │   │   ├── Activity_Log_Date_A
    │   │   └── Activity_Log_Date_B
    │   └── trashed
    ├── DRIVE_CONTRACT_ADMIN # Another Example Shared Drive
    │   ├── active
    │   │   ├── BASE_DE_DATOS_FOLDER
    │   │   │   ├── APU_FORMAT_FILE
    │   │   │   └── EDITIONS_SUBFOLDER
    │   │   └── PROJECT_X
    │   │       └── PROJECT_FOLDER_IU-03
    │   └── trashed
```

### License

This project is licensed under the **Affero General Public License Version 3** (AGPLv3). See the [LICENSE](LICENSE) file for details.
