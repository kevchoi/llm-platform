Performance tuning
Queue depth hit 1, failures are going up, gpu utilization is 76%, gpu mem is 91%. I'm using Amazon T4. getting "Model is overloaded"

decreasing max-total-tokens from the default, 4096, to 2048. but setting max-total-tokens for TGI is causing issues


trying quanitization; using 8 bit quantization

again, got up to like 30 rps before hitting failures

doubled --max-concurrent-requests to 256

same issue; nothing improved but i think gpu utilization went up a little bit more?