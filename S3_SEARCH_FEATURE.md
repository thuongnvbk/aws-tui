# S3 Regex Search Feature

## Overview
Fast, efficient regex-based search functionality for filtering S3 objects in the browser view.

## Features

### 🔍 Regex Pattern Matching
- Full regex support for complex search patterns
- Case-sensitive by default
- Search applies to object names (not full paths for better UX)

### ⚡ Performance Optimizations

#### 1. **Regex Compilation Caching**
```go
// Cache compiled regex patterns to avoid recompilation
var regexCache = make(map[string]*regexp.Regexp)
```
- Compiled patterns are cached in memory
- Maximum cache size: 50 patterns
- FIFO eviction when cache is full
- Thread-safe with RWMutex

#### 2. **Efficient Filtering**
- Pre-allocated slices with estimated capacity
- Single-pass filtering algorithm
- O(n) time complexity where n = number of objects
- Minimal memory allocations

#### 3. **Resource Management**
- Cache size limit prevents memory bloat
- Automatic cache eviction (50% cleared when full)
- No goroutines or channels (synchronous for predictability)
- Minimal CPU usage even with large object lists

## Usage

### Basic Search
1. Press `/` in S3 browser view
2. Type your regex pattern
3. Press `Enter` to apply filter
4. Press `Esc` to cancel or clear filter

### Example Patterns

#### Simple Text Match
```
log
```
Matches: `app.log`, `error.log`, `system.log`

#### File Extension
```
\.json$
```
Matches: `config.json`, `data.json`

#### Date Pattern
```
2024-\d{2}-\d{2}
```
Matches: `backup-2024-01-15.tar.gz`, `log-2024-12-31.txt`

#### Prefix Match
```
^prod-
```
Matches: `prod-app.log`, `prod-config.yaml`

#### Complex Pattern
```
(error|warn|fatal).*\.log$
```
Matches: `error-2024.log`, `warning-system.log`, `fatal-crash.log`

## User Interface

### Search Mode
```
🔍 Search (regex): pattern█

Type regex pattern • Enter: search • Esc: cancel
```

### Active Filter
```
🔍 Filtered: 15 of 1000 objects (press / to search again, Esc to clear)

📁 logs/
📄 app-2024-01-15.log    1.2 MB    2024-01-15 10:30:00
📄 error-2024-01-15.log  45 KB     2024-01-15 10:30:00
...

↑/↓: navigate • Enter: open • /: new search • Esc: clear filter
```

### Error Handling
```
🔍 Search (regex): [invalid(
Error: invalid regex: missing closing ): `[invalid(`

Type regex pattern • Enter: search • Esc: cancel
```

## Performance Characteristics

### Time Complexity
- **Regex compilation**: O(m) where m = pattern length
- **Filtering**: O(n) where n = number of objects
- **Cache lookup**: O(1) average case

### Space Complexity
- **Cache**: O(k) where k = number of cached patterns (max 50)
- **Filtered results**: O(r) where r = number of matches

### Benchmarks (estimated)
- **1,000 objects**: < 10ms
- **10,000 objects**: < 100ms
- **100,000 objects**: < 1s

## Implementation Details

### State Management
```go
// Model fields
s3ObjectsFiltered  []aws.S3Object // Filtered results
s3SearchMode       bool            // Search input active
s3SearchQuery      string          // Current pattern
s3SearchError      string          // Error message
```

### Key Functions

#### `getCompiledRegex(pattern string)`
- Returns cached regex or compiles new one
- Thread-safe with RWMutex
- Automatic cache management

#### `filterS3ObjectsByRegex(objects, pattern)`
- Filters objects using compiled regex
- Returns filtered slice and error
- Optimized for performance

#### `handleS3SearchKeys(msg)`
- Handles keyboard input in search mode
- Real-time error validation
- Immediate feedback

## Behavior

### Filter Persistence
- Filter remains active when navigating within results
- Cleared when:
  - User presses `Esc` in browser view
  - User navigates up/down folders
  - User refreshes the view
  - User switches buckets

### Empty Search
- Empty pattern shows all objects
- Equivalent to clearing the filter

### Invalid Regex
- Error displayed immediately
- Previous filter remains active
- User can correct pattern

## Best Practices

### For Users
1. **Start simple**: Use plain text before complex patterns
2. **Test incrementally**: Build complex patterns step by step
3. **Use anchors**: `^` and `$` for precise matching
4. **Escape special chars**: Use `\` for literal `.`, `*`, etc.

### For Developers
1. **Cache management**: Monitor cache size in production
2. **Pattern validation**: Consider adding pattern suggestions
3. **Performance monitoring**: Log slow searches (>100ms)
4. **User feedback**: Show match count prominently

## Future Enhancements

### Short Term
- [ ] Case-insensitive mode toggle
- [ ] Search history (last 10 patterns)
- [ ] Pattern suggestions/templates
- [ ] Highlight matches in results

### Medium Term
- [ ] Save favorite patterns
- [ ] Multi-field search (size, date, etc.)
- [ ] Fuzzy search option
- [ ] Search across all buckets

### Long Term
- [ ] Advanced query language
- [ ] Saved searches with names
- [ ] Search result export
- [ ] Search analytics

## Troubleshooting

### Search is slow
- **Cause**: Very large object list (>100k)
- **Solution**: Use more specific patterns to reduce initial list

### Pattern doesn't match
- **Cause**: Regex syntax error or wrong pattern
- **Solution**: Test pattern at regex101.com first

### Cache memory concerns
- **Cause**: Many unique patterns
- **Solution**: Cache auto-evicts at 50 patterns

## Code Examples

### Simple Usage
```go
// Enter search mode
m.s3SearchMode = true
m.s3SearchQuery = ""

// Apply filter
filtered, err := filterS3ObjectsByRegex(m.s3Objects, "\.log$")
if err == nil {
    m.s3ObjectsFiltered = filtered
    m.updateS3ObjectsList()
}

// Clear filter
m.s3ObjectsFiltered = nil
m.updateS3ObjectsList()
```

### Custom Pattern
```go
// Search for dated backups
pattern := `backup-\d{4}-\d{2}-\d{2}\.tar\.gz$`
filtered, _ := filterS3ObjectsByRegex(objects, pattern)
```

## Security Considerations

### ReDoS Protection
- No user-controlled regex timeout (Go's regex is safe)
- Cache limits prevent memory exhaustion
- Synchronous execution prevents resource starvation

### Input Validation
- Regex compilation errors caught and displayed
- No code execution risk
- Pattern length not limited (Go handles safely)

## Conclusion

The regex search feature provides:
- ✅ Fast filtering even with large object lists
- ✅ Powerful pattern matching capabilities
- ✅ Efficient resource usage
- ✅ Intuitive user interface
- ✅ Robust error handling

Perfect for finding specific objects in buckets with thousands of files!