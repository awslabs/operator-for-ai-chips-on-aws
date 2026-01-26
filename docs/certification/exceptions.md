# Red Hat Certification Exceptions

This document tracks all exceptions required for Red Hat certification of the AWS Neuron Operator.

## Exception #1: Neuron Monitor Privileged Container

**Component:** Node Metrics DaemonSet  
**File:** `internal/nodemetrics/nodemetrics.go`  
**Image:** `public.ecr.aws/neuron/neuron-monitor:1.3.0`  
**Issue:** Container requires `privileged: true` and `runAsUser: 0`

### Justification

**Technical Requirement:**
The AWS Neuron Monitor requires privileged access to monitor Neuron hardware devices. This is confirmed by AWS's official Kubernetes deployment configuration.

**Evidence from AWS Documentation:**
- **Source:** https://awsdocs-neuron.readthedocs-hosted.com/en/latest/containers/tutorials/k8s-neuron-monitor.html
- **Official AWS YAML:** Uses `securityContext: privileged: true`
- **Purpose:** Hardware monitoring of AWS Inferentia/Trainium chips

**Why Privileged Access is Required:**
1. **Hardware Device Access:** Neuron monitor needs direct access to `/sys` filesystem to read hardware metrics
2. **Device File Access:** Requires access to Neuron device files in `/dev`
3. **System-level Monitoring:** Collects low-level hardware performance metrics
4. **AWS Design:** This is how AWS designed and documents the Neuron monitor deployment

**Security Mitigations:**
1. **Limited Scope:** Only runs on nodes with Neuron hardware (node selector)
2. **Read-Only Operations:** Primarily reads hardware metrics, doesn't modify system
3. **AWS Maintained:** Image is maintained and security-scanned by AWS
4. **Isolated Namespace:** Runs in dedicated namespace with limited RBAC

**Alternative Analysis:**
- **Attempted:** Testing without privileged mode would require Neuron hardware
- **AWS Recommendation:** Official AWS documentation uses privileged mode
- **No Alternative:** No documented way to run Neuron monitor without privileged access

### Red Hat Exception Request

**Exception Type:** Privileged Container  
**Scope:** Node Metrics DaemonSet only  
**Justification:** Hardware monitoring requirement for AWS Neuron devices  
**Risk Assessment:** Low - read-only hardware monitoring, AWS-maintained image  
**Business Impact:** Critical for Neuron device observability and support  

---

## Exception Summary

| Exception | Component | Risk Level | Status |
|-----------|-----------|------------|--------|
| Privileged Container | Neuron Monitor | Low | Pending |

**Total Exceptions:** 1  
**Next Review:** After Red Hat feedback