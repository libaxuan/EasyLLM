<template>
  <GenericAccountView
    title="Cursor 账号管理"
    icon="💻"
    :api="cursorAPI"
    :can-activate="true"
    token-label="Access Token / Cookie"
    token-field="access_token"
    token-placeholder="eyJ..."
  />
</template>
<script setup>
import GenericAccountView from './GenericAccountView.vue'
import { cursorAPI } from '@/api'
</script>
