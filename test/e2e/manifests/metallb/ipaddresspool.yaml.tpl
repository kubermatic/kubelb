# MetalLB IP Address Pool configuration
# Template: ${METALLB_IP_RANGE} is replaced by deploy.sh
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: kubelb
  namespace: kubelb
spec:
  addresses:
    - ${METALLB_IP_RANGE}
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: kubelb
  namespace: kubelb
