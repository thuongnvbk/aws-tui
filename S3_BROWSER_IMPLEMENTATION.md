# S3 Browser Feature Implementation

## Overview
This document describes the implementation of the S3 bucket browsing and object viewing feature for AWS TUI.

## Feature Summary
The S3 browser allows users to:
- Navigate through S3 bucket contents (folders/objects)
- View object metadata (size, modified date, storage class, tags, encryption)
- Preview text-based file contents directly in the TUI
- Download objects to local filesystem
- Generate presigned URLs for sharing
- Navigate using breadcrumb trail

## Implementation Details

### Phase 1: Data Models & AWS Client Methods

#### New Files Created:
- **`internal/aws/s3_objects.go`** (358 lines)
  - `S3Object` struct for representing objects and prefixes
  - `S3ObjectDetail` struct for detailed object information
  - AWS SDK integration methods:
    - `ListObjects()` - List objects with pagination support
    - `GetObjectMetadata()` - Retrieve object metadata and tags
    - `GetObjectContent()` - Fetch object content with size limits
    - `DownloadObject()` - Download to local filesystem
    - `GeneratePresignedURL()` - Create temporary access URLs
  - Helper functions:
    - `IsPreviewableContent()` - Check if file type is previewable
    - `FormatSize()` - Human-readable size formatting
    - `GetObjectName()` - Extract filename from key
    - `GetParentPrefix()` - Navigate up directory tree
    - `BuildBreadcrumb()` - Create navigation breadcrumb

#### Modified Files:
- **`internal/aws/resources.go`**
  - Enhanced `S3Bucket` struct with `Versioning` and `Encryption` fields

### Phase 2: UI State Management

#### Modified Files:
- **`internal/config/config.go`**
  - Enhanced `S3Config` struct with:
    - `MaxObjectsPerPage` - Pagination limit
    - `ContentPreviewSize` - Preview size limit (100KB default)
    - `SupportedPreviewTypes` - List of previewable MIME types
    - `DownloadDirectory` - Default download location
  - Updated `DefaultConfig()` with S3 browsing defaults

- **`config.example.yaml`**
  - Added S3 browsing configuration section with examples

- **`internal/ui/model.go`**
  - Added new view modes:
    - `ViewS3Browser` - Browse objects in bucket
    - `ViewS3ObjectDetail` - View object metadata
    - `ViewS3ObjectContent` - View object content
  - Added S3 state fields to `Model`:
    - `currentBucket` - Active bucket name
    - `currentPrefix` - Current folder prefix
    - `s3Objects` - List of objects in current view
    - `s3BreadcrumbPath` - Navigation breadcrumb
    - `s3ObjectDetail` - Selected object details
    - `s3ContentViewer` - Viewport for content display
  - Added new key bindings:
    - `Browse` - Enter bucket/folder
    - `View` - View object content
    - `Download` - Download object
    - `GoUp` - Navigate up one level
  - Added new message types:
    - `s3ObjectsFetchedMsg`
    - `s3ObjectDetailFetchedMsg`
    - `s3ObjectContentFetchedMsg`
    - `s3DownloadCompleteMsg`

### Phase 3: UI Views & Rendering

#### New Files Created:
- **`internal/ui/s3_browser.go`** (157 lines)
  - `renderS3Browser()` - Main browser view with object list
  - `renderBreadcrumb()` - Navigation breadcrumb display
  - `renderS3ObjectList()` - Formatted object list with icons
  - `renderS3ObjectDetail()` - Object metadata display
  - `renderS3ObjectContent()` - Content viewer with scrolling

#### Modified Files:
- **`internal/ui/view.go`**
  - Integrated S3 views into main `View()` switch statement

### Phase 4: Key Bindings & Navigation Logic

#### New Files Created:
- **`internal/ui/s3_handlers.go`** (317 lines)
  - `fetchS3Objects()` - Async object fetching
  - `fetchS3ObjectDetail()` - Async metadata fetching
  - `fetchS3ObjectContent()` - Async content fetching
  - `downloadS3Object()` - Async download operation
  - `handleS3BrowserKeys()` - Browser view key handling
  - `handleS3ObjectDetailKeys()` - Detail view key handling
  - `handleS3ObjectContentKeys()` - Content view key handling
  - `updateS3ObjectsList()` - Update list with S3 objects

#### Modified Files:
- **`internal/ui/model.go`**
  - Added message handlers for S3 operations
  - Integrated S3 view mode handlers
  - Modified Enter key to open S3 browser for buckets

## User Experience Flow

```
1. User navigates to S3 tab
2. User sees list of S3 buckets
3. User presses Enter on a bucket
   └─> Opens S3 Browser view
4. User navigates folders using Enter
   └─> Updates current prefix and reloads objects
5. User presses Backspace to go up
   └─> Navigates to parent folder
6. User selects an object and presses Enter
   └─> Shows Object Detail view
7. User presses 'v' to view content
   └─> Opens Content Viewer (if previewable)
8. User presses 'd' to download
   └─> Downloads to configured directory
9. User presses 'c' in detail view
   └─> Generates and copies presigned URL
10. User presses Esc to go back
    └─> Returns to previous view
```

## Keyboard Shortcuts

### S3 Browser View
| Key | Action |
|-----|--------|
| `↑/k` | Move up in list |
| `↓/j` | Move down in list |
| `Enter` | Open folder or view object details |
| `Backspace` | Go up one level |
| `v` | View object content (if previewable) |
| `d` | Download object |
| `c` | Copy object key |
| `r` | Refresh current view |
| `Esc` | Return to bucket list |

### Object Detail View
| Key | Action |
|-----|--------|
| `v` | View content (if previewable) |
| `d` | Download object |
| `c` | Generate and copy presigned URL |
| `Esc` | Return to browser |

### Content Viewer
| Key | Action |
|-----|--------|
| `↑/k` | Scroll up |
| `↓/j` | Scroll down |
| `d` | Download full object |
| `Esc` | Return to detail view |

## Configuration Example

```yaml
resources:
  s3:
    enabled: true
    prefixes: []
    # S3 browsing settings
    max_objects_per_page: 1000
    content_preview_size: 102400  # 100 KB
    supported_preview_types:
      - text/plain
      - application/json
      - application/yaml
      - text/csv
      - text/html
      - text/xml
      - application/xml
    download_directory: "~/Downloads"
```

## Technical Highlights

### Performance Optimizations
- Pagination support for large buckets (configurable limit)
- Lazy loading of object details (only when requested)
- Content preview size limits to prevent memory issues
- Efficient breadcrumb navigation

### Error Handling
- Graceful handling of access denied errors
- User-friendly error messages
- Fallback for unsupported file types
- Download directory creation if missing

### Security Considerations
- Respects IAM permissions
- Presigned URLs with 15-minute expiration
- No credential exposure in logs
- Safe path handling for downloads

## Files Modified/Created Summary

### New Files (3)
1. `internal/aws/s3_objects.go` - S3 operations and data models
2. `internal/ui/s3_browser.go` - S3 browser UI components
3. `internal/ui/s3_handlers.go` - S3 event handlers and logic

### Modified Files (5)
1. `internal/aws/resources.go` - Enhanced S3Bucket struct
2. `internal/config/config.go` - S3 browsing configuration
3. `config.example.yaml` - Configuration examples
4. `internal/ui/model.go` - State management and integration
5. `internal/ui/view.go` - View rendering integration

### Total Lines Added
- New code: ~832 lines
- Modified code: ~150 lines
- **Total: ~982 lines**

## Testing Recommendations

### Unit Tests to Add
1. Test S3 object listing with various prefixes
2. Test breadcrumb path building
3. Test content type detection
4. Test size formatting
5. Test parent prefix calculation

### Integration Tests to Add
1. Test with real S3 buckets (using localstack)
2. Test with large buckets (>1000 objects)
3. Test with various file types
4. Test error scenarios (access denied, bucket not found)
5. Test download functionality

### Manual Testing Checklist
- [ ] Navigate into nested folders
- [ ] View object details
- [ ] Preview text files
- [ ] Download objects
- [ ] Generate presigned URLs
- [ ] Test with empty buckets
- [ ] Test with large files
- [ ] Test with special characters in names
- [ ] Test breadcrumb navigation
- [ ] Test error handling

## Future Enhancements

### Short Term
1. Add search/filter within bucket
2. Add sorting options (name, size, date)
3. Add object versioning support
4. Add bulk operations (multi-select)

### Medium Term
1. Add image preview (ASCII art)
2. Add syntax highlighting for code files
3. Add object tagging interface
4. Add lifecycle policy viewer

### Long Term
1. Add object upload functionality
2. Add bucket management (create/delete)
3. Add access control (ACL) management
4. Add CloudFront integration

## Conclusion

The S3 browser feature has been successfully implemented with:
- ✅ Full navigation through bucket hierarchy
- ✅ Object metadata viewing
- ✅ Content preview for text files
- ✅ Download functionality
- ✅ Presigned URL generation
- ✅ Intuitive keyboard-driven interface
- ✅ Comprehensive error handling
- ✅ Configurable behavior

The implementation follows the existing codebase patterns and integrates seamlessly with the current AWS TUI architecture.