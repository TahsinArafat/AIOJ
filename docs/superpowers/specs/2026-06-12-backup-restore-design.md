# Design Specification: Backup & Restore System for AIOJ Admin

This document specifies the design for a built-in backup and restore system in the AIOJ administrator panel. It enables administrators to create, list, download, upload, delete, and restore database dumps and server files directly from the UI.

---

## 1. Storage & Backup Types

All backups are stored on the server filesystem in `/app/backups/`.
Filenames follow the pattern: `aioj_backup_<type>_<timestamp>_<random>.tar.gz`

### Backup Types:
1. **db** (Database Only): Contains a PostgreSQL plain SQL dump of the database (from `pg_dump`).
2. **files** (Files Only): Contains a compressed archive of `testdata/` (problems' test cases) and `media/` (uploaded images/files).
3. **full** (Combined): Contains both the SQL dump and the compressed archive of `testdata/` and `media/`.

### Metadata:
Every backup file is accompanied by a JSON file (e.g. `aioj_backup_<type>_<timestamp>_<random>.json`) containing:
* `filename`: Name of the archive file.
* `type`: One of `"db"`, `"files"`, `"full"`.
* `created_at`: Creation timestamp (RFC3339).
* `size`: Size in bytes.
* `version`: AIOJ version/migration version at time of backup.
* `created_by`: Username of the admin who generated it.

---

## 2. CLI Prerequisites (Docker Image)

To perform safe PostgreSQL backups/restores, the backend Docker container needs standard CLI utilities.
We will add `postgresql-client` and `tar` to the backend Docker image (`Dockerfile`):
```dockerfile
RUN apk add --no-cache ca-certificates tzdata \
    g++ gcc make musl-dev python3 openjdk21-jdk rust cargo nodejs npm bash postgresql-client tar
```

---

## 3. API Endpoints

All endpoints are prefix-matched with `/api/admin/backups` and require the `admin` role middleware.

### 1. List Backups
* **Method**: `GET`
* **URL**: `/api/admin/backups`
* **Response**: `200 OK`
  ```json
  {
    "data": [
      {
        "filename": "aioj_backup_db_20260612_120000_abc123.tar.gz",
        "type": "db",
        "created_at": "2026-06-12T12:00:00Z",
        "size": 1048576,
        "version": "51",
        "created_by": "admin"
      }
    ]
  }
  ```

### 2. Create Backup
* **Method**: `POST`
* **URL**: `/api/admin/backups`
* **Request Body**:
  ```json
  {
    "type": "db"
  }
  ```
* **Response**: `201 Created`
  ```json
  {
    "status": "created",
    "filename": "aioj_backup_db_20260612_120000_abc123.tar.gz"
  }
  ```

### 3. Download Backup
* **Method**: `GET`
* **URL**: `/api/admin/backups/{filename}`
* **Response**: `200 OK` (binary stream with header `Content-Disposition: attachment; filename="..."`)

### 4. Upload Backup
* **Method**: `POST`
* **URL**: `/api/admin/backups/upload`
* **Request Body**: `multipart/form-data` with `file` field.
* **Response**: `201 Created`
  ```json
  {
    "status": "uploaded",
    "filename": "aioj_backup_db_20260612_120000_abc123.tar.gz"
  }
  ```

### 5. Restore Backup
* **Method**: `POST`
* **URL**: `/api/admin/backups/{filename}/restore`
* **Request Body**:
  ```json
  {
    "password": "admin-password"
  }
  ```
* **Response**: `200 OK`
  ```json
  {
    "status": "restored"
  }
  ```

### 6. Delete Backup
* **Method**: `DELETE`
* **URL**: `/api/admin/backups/{filename}`
* **Response**: `200 OK`
  ```json
  {
    "status": "deleted"
  }
  ```

---

## 4. Backend Implementation Flow

### Backup Generation:
1. Validate requested type (`db`, `files`, `full`).
2. Create temporary working directory.
3. If `db` or `full`:
   * Execute `pg_dump` with connection string credentials.
   * Write output to `db.sql`.
4. If `files` or `full`:
   * Package `./testdata` and `./media` into `files.tar.gz`.
5. Package the temporary files (e.g. `db.sql`, `files.tar.gz`) into the final tarball file in `/app/backups/`.
6. Write metadata JSON file.
7. Clean up temporary directory.

### Restore Execution:
1. Verify the requesting user's password using the standard authentication library. If invalid, reject with `401 Unauthorized`.
2. Locate the backup file in `/app/backups/`.
3. Create temporary extraction directory.
4. Extract the backup tarball.
5. If the backup contains `db.sql`:
   * Execute `psql -h <db_host> -p <db_port> -U <db_user> -d <db_name> -f db.sql` (after clearing/dropping existing tables or letting the `--clean --if-exists` flags handle it).
6. If the backup contains `files.tar.gz`:
   * Delete existing contents of `./testdata` and `./media`.
   * Extract `files.tar.gz` contents into their respective directories.
7. Clean up temporary directory.

---

## 5. Frontend UI (`BackupsPanel.tsx`)

### Layout
* **Action Header**: Title, description, and "Refresh" button.
* **Backup Trigger Panel**: A selection area for selecting type (`Database`, `System Files`, `Full Backup`) and a big "Create Backup" button.
* **Upload Area**: Drag-and-drop file uploader for `.tar.gz` backup packages.
* **Backup List Table**:
  * Columns: File Name, Type (Badge), Size, Date Created, Created By, Actions.
  * Actions: Download button, Restore button, Delete button.

### Safety Confirmation Modal
* Displays a stark modal warning about total data replacement.
* Input field: Current Admin's password.
* Action button: "Confirm Destructive Restore" (disabled until password is provided).
* Progress spinner while restore runs.
