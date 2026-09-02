# Agent Crash Remediation Plan

## Executive Summary

This document outlines remediation strategies to prevent agent crashes like the one that occurred on bead `aicodeba-0d66f9a9` (exit code -1, signal -1). The crash pattern indicates **TypeScript build errors cascading to resource exhaustion**, likely triggering the OOM killer.

## Root Cause Analysis Summary

**Primary Root Cause:** TypeScript Build Error Cascading to Resource Exhaustion (75% confidence)

### Key Evidence:
- Exit code -1 (signal -1) → system-level process termination
- Crash occurred 16 seconds AFTER bead closure → post-closure cleanup phase
- TypeScript compilation error found in `event-ribbon.ts:128` (MouseEvent vs Event type mismatch)
- Workspace pattern of OOMKill incidents in acb-index-builder
- 11 claim/release cycles with 4 recorded failures → persistent resource/build issues

### Failure Sequence:
1. Agent implements feature
2. TypeScript build fails (type mismatch)
3. Build process consumes resources during repeated attempts
4. System terminates process (signal -1) → OOM or resource exhaustion
5. Bead already closed, crash during cleanup

## Remediation Strategy

### 1. Build Verification (Immediate Priority)

**Problem:** Agents close beads without verifying that builds succeed.

**Solution:** Add pre-build verification as a bead closure requirement.

**Implementation:**
```typescript
// Add to build process
npm run build --dry-run  // TypeScript check only
npm run type-check       // Separate type checking
```

**Benefits:**
- Catches type errors before bead closure
- Prevents cascading build failures
- Faster feedback loop

### 2. Resource Monitoring & Limits (Medium Priority)

**Problem:** No visibility into agent resource consumption before OOM.

**Solution:** Add resource monitoring and enforce limits.

**Implementation Options:**
```yaml
# Kubernetes deployment (for worker pods)
resources:
  requests:
    cpu: "100m"
    memory: "256Mi"
  limits:
    cpu: "500m"      # Add CPU limit
    memory: "512Mi"  # Already set
```

**Benefits:**
- Predictable resource usage
- Prevents runaway processes
- Better resource allocation

### 3. Timeout Mechanisms (High Priority)

**Problem:** Build processes can hang indefinitely, consuming resources.

**Solution:** Add timeouts at multiple levels.

**Implementation:**
```bash
# Package.json scripts
{
  "build": "timeout 60s tsc && timeout 120s vite build",
  "type-check": "timeout 30s tsc --noEmit"
}
```

**Benefits:**
- Prevents indefinite hangs
- Forces agent to handle timeout gracefully
- Reduces resource waste

### 4. Retry Logic with Exponential Backoff (Low Priority)

**Problem:** Agents retry failed builds immediately, consuming resources.

**Solution:** Implement exponential backoff for build retries.

**Implementation:**
```typescript
// Pseudocode for retry logic
async function buildWithRetry(maxRetries = 3) {
  for (let attempt = 0; attempt < maxRetries; attempt++) {
    try {
      await build();
      return;
    } catch (error) {
      if (attempt === maxRetries - 1) throw error;
      const delay = Math.pow(2, attempt) * 1000; // 1s, 2s, 4s
      await sleep(delay);
    }
  }
}
```

**Benefits:**
- Reduces resource pressure during failures
- Allows transient issues to resolve
- Better system stability

### 5. Type Safety Enforcement (Immediate Priority)

**Problem:** TypeScript strict type checking violations slip through to production.

**Solution:** Enforce stricter type checking during development.

**Implementation:**
```json
// tsconfig.json
{
  "compilerOptions": {
    "strict": true,
    "noImplicitAny": true,
    "strictNullChecks": true,
    "strictFunctionTypes": true,
    "strictBindCallApply": true,
    "strictPropertyInitialization": true,
    "noImplicitThis": true,
    "alwaysStrict": true
  }
}
```

**Benefits:**
- Catches type errors at compile time
- Prevents runtime type errors
- Better code quality

## Proposed Changes

### Immediate (High Impact, Low Effort)

1. **Add build verification to bead closure workflow**
   - Run `npm run type-check` before closing implementation beads
   - Make build failure a blocker for bead closure
   - Status: **Ready to implement**

2. **Add timeout to npm build scripts**
   - Modify `web/package.json` build scripts
   - Add timeout wrapper to prevent hangs
   - Status: **Ready to implement**

3. **Enable strict TypeScript checking**
   - Update `web/tsconfig.json` with strict options
   - Fix any resulting type errors
   - Status: **Requires testing**

### Medium (High Impact, Medium Effort)

4. **Add resource monitoring to agent workers**
   - Add memory/CPU monitoring to worker pods
   - Alert on approaching limits
   - Status: **Requires infrastructure changes**

5. **Implement exponential backoff for retries**
   - Add retry logic to agent build process
   - Configure reasonable backoff intervals
   - Status: **Requires agent framework changes**

### Long-term (High Impact, High Effort)

6. **Add pre-commit hooks for type checking**
   - Run `tsc --noEmit` on pre-commit
   - Prevent type errors from entering codebase
   - Status: **Requires workflow changes**

7. **Add CI/CD build verification**
   - Automatic build check on bead closure
   - Block merge if build fails
   - Status: **Requires CI/CD integration**

## Implementation Priority

### Phase 1: Immediate Fixes (This Week)
- ✅ Add timeout to build scripts
- ✅ Enable strict TypeScript checking
- ✅ Add type-check step to bead closure workflow

### Phase 2: Process Improvements (Next Week)
- Implement exponential backoff for retries
- Add resource monitoring alerts
- Document build verification requirements

### Phase 3: Infrastructure Improvements (Next Month)
- Add pre-commit hooks
- Integrate with CI/CD pipeline
- Add comprehensive resource limits

## Success Metrics

- **Crash Rate:** Reduce agent crashes from 4 per bead to <0.1 per bead
- **Build Success Rate:** Increase to >95% on first attempt
- **Resource Efficiency:** Reduce memory usage during builds by 50%
- **Type Safety:** Zero TypeScript strict type errors in production

## Related Issues

- Bead `aicodeba-0d66f9a9`: Original crash event
- Bead `aicodeba-3f4efb1a`: Root cause analysis
- Bead `aicodeba-41db5914`: Crash diagnostics
- Commits `a772aab`, `1b399a1`, `7e9d1af`: Previous OOMKill incidents

## References

- TypeScript Strict Mode: https://www.typescriptlang.org/tsconfig#strict
- Node.js Timeout Handling: https://nodejs.org/api/globals.html#class_timeout
- Kubernetes Resource Management: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/

---
**Document Created:** 2026-09-02T02:00:00Z  
**Author:** claude-code-glm-4.7-glm-acb  
**Status:** Ready for Implementation
