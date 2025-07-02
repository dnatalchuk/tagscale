# TagScale 🚀

> Cloud cost insights without perfect tagging

![License: BSL](https://img.shields.io/badge/license-BSL-blue.svg)

TagScale helps engineering teams understand **where their cloud costs go** — even if your tagging is messy or incomplete.

✅ **Zero-effort cost attribution**  
✅ Works even with missing or inconsistent tags  
✅ Clean dashboards for small teams  
✅ Slack and email digests for cost visibility

---

## ✨ Why TagScale?

Traditional FinOps tools demand perfect tagging or enterprise-level budgets. TagScale takes a different approach:

- Infers cost ownership using resource names, patterns, and heuristics
- Highlights untagged spend and potential savings
- Designed for small-to-mid-sized teams who want clarity without complexity

No more cloud invoices that look like hieroglyphics.

---

## 🎯 MVP Scope

The MVP focuses on **read-only cost analysis**:

- Pull AWS Cost Explorer data
- Group costs by:
  - Service
  - Account
  - Region
  - Tags (when available)
- Infer missing tag values
- Visualize reports in a Next.js dashboard
- Generate optional Slack/email digests

Future roadmap includes:
- GCP and Azure support
- Policy-driven tagging recommendations
- CI/CD integration

---
