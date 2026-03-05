# OTLP Header Tenant Extraction Changes

**File to modify:** `pkg/agent/collectors/events/templates/events-logger.alloy.template`

---

## CHANGE 1: Add `include_metadata = true` to OTLP receiver

**Location:** Lines ~78-88 (OTLP receiver block)

### BEFORE:
```alloy
// OTLP receiver for traces
otelcol.receiver.otlp "traces" {
	grpc {
		endpoint = "0.0.0.0:4317"
	}
	http {
		endpoint = "0.0.0.0:4318"
	}

	output {
		traces = [otelcol.processor.k8sattributes.default.input]
	}
}
```

### AFTER:
```alloy
// OTLP receiver for traces
otelcol.receiver.otlp "traces" {
	grpc {
		endpoint = "0.0.0.0:4317"
	}
	http {
		endpoint = "0.0.0.0:4318"
	}

	output {
		traces = [otelcol.processor.k8sattributes.default.input]
	}

	// Enable metadata forwarding so headers are passed downstream for tenant extraction
	include_metadata = true
}
```

---

## CHANGE 2: Update k8sattributes processor - PART A (tag_name)

**Location:** Lines ~100 in k8sattributes processor

### BEFORE:
```alloy
		label {
			key = "observability.giantswarm.io/tenant"
			tag_name = "giantswarm.tenant"
		}
```

### AFTER:
```alloy
		label {
			key = "observability.giantswarm.io/tenant"
			tag_name = "giantswarm.tenant.from-label"
		}
```

---

## CHANGE 3: Update k8sattributes processor - PART B (output)

**Location:** Lines ~107-110 in k8sattributes processor

### BEFORE:
```alloy
	output {
		traces = [otelcol.processor.transform.default.input]
	}
}
```

### AFTER:
```alloy
	output {
		traces = [otelcol.processor.attributes.default.input]
	}
}
```

---

## CHANGE 4: Add new attributes processor

**Location:** Insert right after the k8sattributes closing brace (before "// Add cluster metadata to traces" comment)

### ADD THIS BLOCK:
```alloy

// Extract tenant from X-Scope-OrgID header and promote to resource attribute
otelcol.processor.attributes "default" {
	action {
		key = "giantswarm.tenant.from-header"
		from_context = "request_metadata"
		pattern = "x-scope-orgid"
		action = "extract"
	}

	output {
		traces = [otelcol.processor.transform.default.input]
	}
}
```

---

## CHANGE 5: Update transform processor - PART A (comment)

**Location:** Line ~115 before the transform processor

### BEFORE:
```alloy
// Add cluster metadata to traces
otelcol.processor.transform "default" {
```

### AFTER:
```alloy
// Add cluster metadata to traces and resolve tenant priority: header > label > default
otelcol.processor.transform "default" {
```

---

## CHANGE 6: Update transform processor - PART B (add tenant resolution)

**Location:** Lines ~120-127 in transform processor statements

### BEFORE:
```alloy
	trace_statements {
		context = "resource"
		statements = [
			`set(attributes["giantswarm.cluster.id"], "{{ .ClusterID }}")`,
			`set(attributes["giantswarm.cluster.type"], "{{ .ClusterType }}")`,
			`set(attributes["giantswarm.cluster.organization"], "{{ .Organization }}")`,
			`set(attributes["giantswarm.cluster.provider"], "{{ .Provider }}")`,
		]
	}
```

### AFTER:
```alloy
	trace_statements {
		context = "resource"
		statements = [
			`set(attributes["giantswarm.cluster.id"], "{{ .ClusterID }}")`,
			`set(attributes["giantswarm.cluster.type"], "{{ .ClusterType }}")`,
			`set(attributes["giantswarm.cluster.organization"], "{{ .Organization }}")`,
			`set(attributes["giantswarm.cluster.provider"], "{{ .Provider }}")`,
			// Tenant resolution: prefer header value over label value
			`attributes["giantswarm.tenant"] = (attributes["giantswarm.tenant.from-header"] != nil) ? attributes["giantswarm.tenant.from-header"] : attributes["giantswarm.tenant.from-label"]`,
		]
	}
```

---

## After Applying All Changes

For each branch, run:

```bash
# 1. Make sure you're in the correct branch
git status

# 2. Verify your changes look correct
git diff pkg/agent/collectors/events/templates/events-logger.alloy.template

# 3. Regenerate golden files
make generate-golden-files

# 4. Stage all changes
git add -A

# 5. Commit with proper message
git commit -m "fix(otlp): add include_metadata and header-based tenant extraction

- Enable include_metadata in OTLP receiver for metadata forwarding
- Add attributes processor to extract X-Scope-OrgID header
- Update k8sattributes to use giantswarm.tenant.from-label
- Implement tenant resolution: header > label
- Regenerate golden test files"

# 6. Push to remote
git push origin <branch-name>
```

---

## Summary of Changes

The pipeline transforms from:
```
receiver (no metadata) → k8sattributes → transform → filter
```

To:
```
receiver (include_metadata=true) → k8sattributes (from-label) → attributes (from-header) → transform (resolves) → filter
```

This enables:
1. **Metadata forwarding**: Headers are passed through the pipeline
2. **Header extraction**: X-Scope-OrgID header is extracted to `giantswarm.tenant.from-header`
3. **Label extraction**: Pod labels are extracted to `giantswarm.tenant.from-label`
4. **Priority resolution**: Header value takes precedence over label value when both exist
