<template>
  <GenericAccountView
    title="Claude 账号管理"
    icon="🧠"
    :api="claudeAPI"
    :can-activate="false"
    token-label="Session Key"
    token-field="session_key"
    token-placeholder="sk-ant-..."
  />
</template>
<script setup>
import GenericAccountView from './GenericAccountView.vue'
import { claudeAPI } from '@/api'
</script>
