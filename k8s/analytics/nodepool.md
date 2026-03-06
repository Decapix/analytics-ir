# Analytics node pool notes

Create a dedicated node pool labeled for analytics workloads and optionally tainted:

- Label: `workload=analytics`
- Taint (optional): `workload=analytics:NoSchedule`

All analytics manifests in this directory include `nodeSelector` and `tolerations` for this pool.
