# Red Hat Certification Backlog - CONSOLIDATED

**Status:** ⚠️ BLOCKED - Critical fixes required  
**Last Updated:** January 2025  
**Validation:** ✅ Official Red Hat Documentation + Source Code Analysis

## 📊 Executive Summary

| Category | Status | Count | Effort |
|----------|--------|-------|--------|
| **CRITICAL BLOCKERS** | ❌ Must Fix | 3 items | 2-3 weeks |
| **NEEDS VERIFICATION** | ⚠️ Should Verify | 1 item | 30 minutes |
| **NOT REQUIRED** | ❌ Not Needed | 4 items | - |
| **COMPLIANT** | ✅ Already Done | 13 items | - |

**Total Certification Timeline:** 2-3 weeks (due to container prerequisites)

---

## 🔴 CRITICAL BLOCKERS (Must Address to Proceed)

### P0-1: Container Certification Prerequisites (BLOCKING)
**Source:** Official Red Hat Documentation p.72  
**Effort:** 2-3 weeks  
**Status:** ❌ NOT STARTED

**Requirement:** "All containers referenced in an Operator Bundle must already be certified and published in the Red Hat Ecosystem Catalog prior to beginning to certify an Operator Bundle."

**Impact:** Cannot certify operator until all 5 external containers are certified first

**Action Required:**
1. Set up Red Hat Partner Connect Portal
2. Certify each external container image:
   - `ghcr.io/awslabs/kmod-with-kmm-for-ai-chips-on-aws/neuron-driver:*`
   - `public.ecr.aws/neuron/neuron-device-plugin:*`
   - `public.ecr.aws/eks-distro/kubernetes/kube-scheduler:*` (or replace)
   - `public.ecr.aws/neuron/neuron-scheduler:*`
   - Node metrics image (TBD)

---

### P0-2: Prohibited Labels (BLOCKING)
**Source:** Red Hat Preflight Tests - HasNoProhibitedLabels check FAILED  
**File:** `Dockerfile`  
**Status:** ❌ FAILED - Published image has Red Hat trademark violations in labels

**Published image labels (from UBI base):**
- `name="ubi9-minimal"` (should be operator name)
- `vendor="Red Hat, Inc."` (should be "Amazon Web Services")
- `maintainer="Red Hat, Inc."` (should be AWS contact)

**Dockerfile has correct labels (lines 20-26):**
```dockerfile
LABEL name="AWS Neuron Operator" \
      maintainer="https://github.com/awslabs/operator-for-ai-chips-on-aws/issues" \
      vendor="Amazon Web Services" \
      version="0.1.2" \
      release="1" \
      summary="Operator for AWS Neuron devices on OpenShift" \
      description="Automates enabling AWS Neuron devices (Inferentia/Trainium) on OpenShift clusters. Deploys kernel modules via KMM, device plugins, custom scheduler, and node metrics."
```

**Issue:** Published image was built without these labels or they were overridden

**Action Required:** Rebuild and republish image with current Dockerfile

---

### P0-3: Missing /licenses Directory (BLOCKING)
**Source:** Red Hat Preflight Tests - HasLicense check FAILED  
**File:** `Dockerfile`  
**Status:** ❌ FAILED - /licenses directory in Dockerfile but NOT in published image `public.ecr.aws/os-partners/neuron-openshift/operator:v0.1.3`

**Dockerfile has it (lines 33-35):**
```dockerfile
RUN mkdir -p /licenses
COPY LICENSE /licenses/
COPY NOTICE /licenses/
```

**Issue:** Published image was built without these lines or from old Dockerfile version

**Action Required:** Rebuild and republish image with current Dockerfile

---

### P0-4: Rebuild Published Image (BLOCKING)
**Source:** Red Hat Preflight Tests - 2 checks FAILED  
**File:** Published image `public.ecr.aws/os-partners/neuron-openshift/operator:v0.1.3`  
**Effort:** 30 minutes  
**Status:** ❌ FAILED - Published image missing labels and /licenses

**Failed Checks:**
1. HasLicense - /licenses directory not found in published image
2. HasNoProhibitedLabels - Labels contain Red Hat trademarks

**Root Cause:** Published image was built from old Dockerfile or build process didn't include current changes

**Action Required:** Rebuild and republish image using current Dockerfile (which has correct labels and /licenses)

---

### ~~P0-5: Security Vulnerabilities~~ ✅ DONE
**Source:** Red Hat Preflight Tests  
**File:** `Dockerfile`  
**Status:** ✅ COMPLETED - Using UBI9 minimal:9.7 (pinned version) in Dockerfile:18

**Current Implementation:**
```dockerfile
FROM registry.access.redhat.com/ubi9/ubi-minimal:9.7
```

**Note:** Vulnerabilities will be scanned by Red Hat during certification. Using pinned UBI version is best practice.

---

### P0-6: Node Metrics Privileged Container (BLOCKING)
**Source:** Official Red Hat Documentation p.45 - "Privileged containers might create a security risk"  
**File:** `internal/nodemetrics/nodemetrics.go:91-93`  
**Effort:** 1-2 hours  
**Status:** ⚠️ REQUIRES JUSTIFICATION

**Current Issue:**
```go
SecurityContext: &v1.SecurityContext{
    Privileged: ptr.To[bool](true),    // Runs privileged
    RunAsUser:  ptr.To[int64](0),      // Runs as root
}
```

**Red Hat Requirement:** Privileged containers are allowed but require justification during certification review. From official docs: "If your product's functionality requires root access, you must select the privileged option, before running the preflight tool. This setting is subject to Red Hat review."

**Action Required:**
1. Document why privileged access is needed (likely for neuron-monitor to access hardware)
2. Be prepared to justify during Red Hat review
3. Alternative: Test if specific capabilities can replace privileged mode

---

### ~~P2-1: Scheduler Extension Missing SecurityContext~~ ❌ NOT REQUIRED
**Source:** Code Analysis (NOT from official Red Hat requirements)  
**File:** `internal/customscheduler/customscheduler.go:147`  
**Status:** ❌ NOT REQUIRED - No explicit SecurityContext requirement in official Red Hat certification docs

**Analysis:** 
- Official Red Hat documentation does NOT mandate SecurityContext fields in operator code
- OpenShift applies `restricted` SCC by default, which handles security at platform level
- The scheduler extension container will inherit platform security policies
- Red Hat docs only require declaring privileged mode IF needed (Component Details form)

**Conclusion:** This is a best practice but NOT a certification blocker per official requirements

---

## 🟢 NEEDS VERIFICATION (Should Check)

### P1-3: EKS Scheduler Image (Non-UBI)
**File:** Default configuration  
**Effort:** 30 minutes  
**Status:** ⚠️ NEEDS REVIEW

**Current:** `public.ecr.aws/eks-distro/kubernetes/kube-scheduler:*`  
**Consideration:** Using OpenShift's native scheduler `registry.redhat.io/openshift4/ose-kube-scheduler:v4.14`

**Analysis:** This is user-configurable via DeviceConfig. Not a blocker, but using OpenShift's scheduler may be preferred for better integration.

---

## ❌ NOT REQUIRED (Based on Official Requirements)

### ~~P2-1: Scheduler Extension Missing SecurityContext~~
**Source:** Code Analysis (NOT from official Red Hat requirements)  
**File:** `internal/customscheduler/customscheduler.go:147`  
**Status:** ❌ NOT REQUIRED - No explicit SecurityContext requirement in official Red Hat certification docs

**Analysis:** 
- Official Red Hat documentation does NOT mandate SecurityContext fields in operator code
- OpenShift applies `restricted` SCC by default, which handles security at platform level
- The scheduler extension container will inherit platform security policies
- Red Hat docs only require declaring privileged mode IF needed (Component Details form)

**Conclusion:** This is a best practice but NOT a certification blocker per official requirements

---

### ~~P2-2: Incomplete Custom Scheduler SecurityContext~~
**File:** `internal/customscheduler/customscheduler.go:118`  
**Status:** ❌ NOT REQUIRED - No explicit SecurityContext requirement in official Red Hat certification docs

**Current:**
```go
SecurityContext: &corev1.SecurityContext{Privileged: ptr.To[bool](false)}
```

**Analysis:** Setting `Privileged: false` is sufficient. Additional fields are best practices but not certification requirements.

---

### ~~P2-3: Missing Security Hardening~~
**Files:** All SecurityContext blocks  
**Status:** ❌ NOT REQUIRED - Not mentioned in official Red Hat certification requirements

**Analysis:** These are Kubernetes security best practices but NOT Red Hat certification requirements. OpenShift's SCC system handles this at the platform level.

---

## ✅ ALREADY COMPLIANT

### ✅ Non-Root User Implementation
**File:** `Dockerfile:37-40`  
**Status:** ✅ COMPLIANT

```dockerfile
RUN ["groupadd", "--system", "-g", "201", "aws-neuron"]
RUN ["useradd", "--system", "-u", "201", "-g", "201", "-s", "/sbin/nologin", "aws-neuron"]
USER 201:201
```

### ✅ UBI Base Image Usage
**File:** `Dockerfile:18`  
**Status:** COMPLIANT

```dockerfile
FROM registry.access.redhat.com/ubi9/ubi-minimal:9.4
```

### ✅ No Modified Red Hat Files
**Status:** COMPLIANT - Only user/group creation (allowed)

### ✅ Unique Tag
**File:** Build process  
**Status:** ✅ PASSED - Preflight HasUniqueTag check passed for `v0.1.3`

### ✅ Layer Count
**Status:** ✅ PASSED - Preflight LayerCountAcceptable check passed

### ✅ No Prohibited Packages
**Status:** ✅ PASSED - Preflight HasNoProhibitedPackages check passed

### ✅ Run As Non-Root
**Status:** ✅ PASSED - Preflight RunAsNonRoot check passed

### ✅ No Kernel Packages
**Status:** COMPLIANT - Using ubi-minimal

### ✅ Image Naming
**Status:** COMPLIANT - No Red Hat marks in name

---

## 🚀 Implementation Plan

### Phase 1: Operator Image Fixes (30 minutes)
**Blockers:** P0-2, P0-3, P0-4 (rebuild), P0-6 (document)

1. **Rebuild Published Image** (15 minutes)
   - Current Dockerfile already has correct labels (lines 20-26)
   - Current Dockerfile already has /licenses directory (lines 33-35)
   - Rebuild: `make docker-build IMG=public.ecr.aws/os-partners/neuron-openshift/operator:v0.1.4`
   - Push: `make docker-push IMG=public.ecr.aws/os-partners/neuron-openshift/operator:v0.1.4`
   - Run preflight to verify

2. **Document Privileged Container** (15 minutes)
   - Document why node-metrics needs privileged access (hardware access)
   - Prepare justification for Red Hat review

**Deliverable:** Operator image passing all preflight checks

---

### Phase 2: Container Prerequisites (2-3 weeks)
**Blockers:** P0-1

1. **Set Up Red Hat Partner Connect Portal** (1-2 days)
   - Create account and product listing
   - Set up certification pipeline

2. **Certify External Container Images** (2-3 weeks)
   - Verify UBI compliance of external images
   - Rebuild non-compliant images on UBI base
   - Submit each image for certification
   - Wait for Red Hat approval

3. **Address Node Metrics Privileged Container** (included in Phase 1)
   - Already documented in Phase 1
   - Will be reviewed by Red Hat during certification

**Deliverable:** All containers certified and published

---

### Phase 3: Operator Certification (1 week)
1. **Submit Operator Bundle** for certification
2. **Address any Red Hat feedback**
3. **Publish to Red Hat Ecosystem Catalog**

**Deliverable:** Certified operator available in OperatorHub

---

## 🧪 Testing Checklist

After Phase 1 fixes:

```bash
# 1. Build image
make docker-build IMG=<registry>/operator:v1.0.0-$(date +%Y%m%d)

# 2. Verify fixes
docker run --rm <image> ls -la /licenses  # Should show LICENSE, NOTICE
docker run --rm <image> id                # Should show uid=201
docker history <image> | wc -l            # Should be < 40
docker inspect <image> | grep -A 20 Labels # Should show all required labels

# 3. Run Red Hat Preflight (when available)
preflight check container <image> --certification-project-id=<id>

# 4. Deploy and test functionality
make deploy IMG=<image>
kubectl get pods -n ai-operator-on-aws  # All pods should be running
```

---

## 📁 File Cleanup

**Delete these validation files after consolidation:**
- `RedHatCertificationBacklog_ValidationReport.md` ❌
- `RedHatCertificationBacklog_OfficialValidation.md` ❌

**Keep this file as the single source of truth:**
- `RedHatCertificationBacklog.md` ✅ (this file)

---

## 📞 Next Steps

1. **Immediate (Today):** Implement Phase 1 fixes (1 day effort)
2. **This Week:** Set up Red Hat Partner Connect Portal
3. **Next 2-3 weeks:** Work on container certification prerequisites
4. **Month End:** Submit operator for certification

**Questions/Blockers:** Create GitHub issues for any implementation questions

---

**Validation Sources:**
- ✅ Official Red Hat Software Certification Guide (198 pages)
- ✅ Direct source code analysis
- ✅ Red Hat Preflight tool requirements
- ✅ OpenShift security best practices