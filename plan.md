# Multi-Format Log Support Plan

## Overview

Extend the S3-EdgeDelta streamer to support multiple log formats, starting with Zscaler (current) and Cisco Umbrella logs. The system will use explicit configuration with auto-detection fallback.

## Current State Analysis

### Supported Format: Zscaler
- **File Format**: JSONL (newline-delimited JSON)
- **Compression**: Gzipped
- **Filename Pattern**: `<unix_timestamp>_<id>_<id>_<seq>[.gz]`
- **Partitioning**: Hive-style (`year=YYYY/month=M/day=D/`)
- **Content**: Zscaler NSS web logs
- **Processing**: Lines sent as-is to EdgeDelta

### Target Format: Cisco Umbrella
- **File Format**: CSV with headers
- **Compression**: Gzipped
- **Filename Pattern**: `<year>-<month>-<day>-<hour>-<minute>-<xxxx>.csv.gz`
- **Partitioning**: Date folders (`<year>-<month>-<day>/`)
- **Content**: DNS, proxy, and audit logs
- **Processing**: Raw CSV rows sent to EdgeDelta (no JSON conversion needed)

## Requirements

### Functional Requirements
1. **Format Detection**: Explicit configuration with auto-detection fallback
2. **Timestamp Parsing**: Support both Unix timestamp and date-time filename formats
3. **Content Processing**: Handle JSONL and CSV formats appropriately
4. **Backward Compatibility**: Existing Zscaler deployments continue working
5. **Configuration**: Simple config option to select log format

### Non-Functional Requirements
1. **Performance**: Minimal overhead for format detection/processing
2. **Reliability**: Robust error handling for format mismatches
3. **Maintainability**: Clean architecture for adding future formats
4. **Observability**: Clear logging/metrics for format detection and processing

## Implementation Approach

### Architecture Pattern: Strategy Pattern
```go
type LogFormat interface {
    ParseTimestamp(filename string) (int64, error)
    ProcessContent(line []byte, isFirstLine bool) ([]byte, error)
    GetContentType() string
    DetectFromFilename(filename string) bool
    DetectFromContent(sample []byte) bool
}
```

### Configuration Design
```yaml
processing:
  log_format: "zscaler"  # Options: "zscaler", "cisco_umbrella", "auto"
```

### Detection Logic
1. **Primary**: Use configured `log_format` value
2. **Auto-detection**: When set to "auto", detect from filename/content patterns
3. **Fallback**: Default to Zscaler for backward compatibility

## Implementation Phases

### Phase 1: Core Infrastructure (Week 1)
**Goal**: Establish the format abstraction and configuration

**Tasks:**
1. Define `LogFormat` interface and implementations
2. Add configuration parsing for `log_format` option
3. Create ZscalerFormat implementation (wrap existing logic)
4. Create CiscoUmbrellaFormat implementation
5. Add format registry and factory
6. Update scanner to use format-specific timestamp parsing
7. Update worker to use format-specific content processing

**Deliverables:**
- Format interface and implementations
- Configuration parsing
- Updated scanner and worker
- Unit tests for format detection

**Risks:**
- Breaking existing functionality
- Complex interface design

### Phase 2: Cisco Umbrella Support (Week 2)
**Goal**: Full Cisco Umbrella log processing

**Tasks:**
1. Implement Cisco timestamp parsing (`2025-10-13-14-30-xxxx.csv.gz`)
2. Implement CSV header skipping
3. Update S3 prefix generation for date-based partitioning
4. Add Cisco-specific file pattern detection
5. Update content-type handling (text/plain vs application/x-ndjson)
6. Test with sample Cisco Umbrella files

**Deliverables:**
- Working Cisco Umbrella log processing
- Updated documentation
- Integration tests

**Risks:**
- Cisco log schema variations (v1-v13)
- Date-based partitioning differences

### Phase 3: Auto-Detection & Polish (Week 3)
**Goal**: Robust auto-detection and production readiness

**Tasks:**
1. Implement auto-detection logic
2. Add comprehensive error handling
3. Update metrics and logging for format information
4. Performance optimization
5. Documentation updates
6. Migration guide for existing users

**Deliverables:**
- Auto-detection functionality
- Updated README and configuration docs
- Performance benchmarks
- Migration documentation

**Risks:**
- False positives in auto-detection
- Performance regression

### Phase 4: Testing & Validation (Week 4)
**Goal**: Comprehensive testing and production validation

**Tasks:**
1. Unit tests for all format implementations
2. Integration tests with real log files
3. Performance testing with large datasets
4. End-to-end testing with EdgeDelta
5. Load testing with mixed formats
6. Documentation review and updates

**Deliverables:**
- Complete test suite
- Performance reports
- Production deployment guide
- Updated README with multi-format examples

**Risks:**
- Edge case failures
- Performance issues at scale

## Code Changes Summary

### Files to Modify
1. **internal/scanner/scanner.go**: Update timestamp parsing to use format-specific logic
2. **internal/worker/http_pool.go**: Update content processing for header skipping
3. **internal/output/http_sender.go**: Update content-type based on format
4. **cmd/streamer/main_http.go**: Add format configuration and initialization
5. **internal/config/config.go**: Add log_format configuration field

### Files to Create
1. **internal/formats/format.go**: Format interface and registry
2. **internal/formats/zscaler.go**: Zscaler format implementation
3. **internal/formats/cisco_umbrella.go**: Cisco Umbrella format implementation
4. **internal/formats/auto.go**: Auto-detection logic

### Configuration Changes
```yaml
# New field in config.yaml
processing:
  log_format: "zscaler"  # "zscaler", "cisco_umbrella", or "auto"
```

## Testing Strategy

### Unit Tests
- Format detection accuracy
- Timestamp parsing for both formats
- Content processing (header skipping, etc.)
- Error handling for malformed files

### Integration Tests
- End-to-end processing of sample files
- Mixed format scenarios
- Configuration validation
- Error recovery

### Performance Tests
- Processing throughput comparison
- Memory usage with large files
- CPU overhead of format detection

### Compatibility Tests
- Existing Zscaler deployments continue working
- Backward compatibility with old configs
- Migration path validation

## Migration Considerations

### For Existing Users
- **Zero breaking changes**: Default behavior unchanged
- **Opt-in upgrade**: Set `log_format: "zscaler"` explicitly or use "auto"
- **Gradual migration**: Can migrate deployments individually

### Deployment Strategy
1. **Staged rollout**: Test with Cisco logs first
2. **Feature flags**: Can disable new functionality if issues arise
3. **Monitoring**: Add metrics for format detection success/failure
4. **Rollback plan**: Easy reversion to single-format version

## Success Criteria

1. **Functionality**: Both Zscaler and Cisco Umbrella logs process correctly
2. **Performance**: <5% overhead for format detection/processing
3. **Reliability**: >99.9% format detection accuracy
4. **Usability**: Clear configuration and error messages
5. **Maintainability**: Easy to add new log formats in future

## Future Extensions

The architecture supports easy addition of:
- CrowdStrike logs
- Palo Alto logs
- Custom enterprise formats
- JSONL variants with different schemas

## Timeline & Resources

- **Duration**: 4 weeks
- **Team**: 1-2 developers
- **Dependencies**: Sample log files for testing
- **Risk Mitigation**: Extensive testing, gradual rollout

## Approval & Next Steps

1. Review and approve plan
2. Gather sample Cisco Umbrella log files
3. Begin Phase 1 implementation
4. Weekly progress reviews