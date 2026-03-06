<template>
  <GenericAccountView
    title="Antigravity 账号管理"
    icon="🚀"
    :api="antigravityAPI"
    :can-activate="true"
    token-label="Access Token"
    token-field="access_token"
  />
</template>
<script setup>
import GenericAccountView from './GenericAccountView.vue'
import { antigravityAPI } from '@/api'
</script>
