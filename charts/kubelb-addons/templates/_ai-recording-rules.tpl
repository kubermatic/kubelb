{{/*
Recording rule groups for AI usage showback. This is the stable metrics
contract consumers (the KubeLB Dashboard, customer billing pipelines) bind
to: kubelb:* series survive upstream agentgateway metric renames, which only
cost a rule edit here.

Rules record hourly increases rather than calendar windows because recording
rules cannot parametrize "since Monday"; consumers sum the hourly series over
whatever window they render. Input/output token splits are preserved so
token-type budgets can land later without a contract break.

USD rules are deliberately absent until the data-plane cost catalog story
lands; the cost series name cannot be verified without one.
*/}}
{{- define "kubelb-addons.aiRecordingRuleGroups" -}}
groups:
  - name: kubelb-ai-usage
    interval: {{ .Values.aiRecordingRules.interval }}
    rules:
      - record: kubelb:ai_tokens:increase1h
        expr: sum by (tenant_id, key_id, gen_ai_token_type) (increase(agentgateway_gen_ai_client_token_usage_sum{tenant_id!=""}[1h]))
      - record: kubelb:ai_requests:increase1h
        expr: sum by (tenant_id, key_id) (increase(agentgateway_gen_ai_client_token_usage_count{tenant_id!="",gen_ai_token_type="input"}[1h]))
      - record: kubelb:ai_tokens_by_model:increase1h
        expr: sum by (tenant_id, gen_ai_system, gen_ai_response_model) (increase(agentgateway_gen_ai_client_token_usage_sum{tenant_id!=""}[1h]))
{{- end -}}
