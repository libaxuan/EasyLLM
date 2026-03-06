<template>
  <GenericAccountView
    title="Windsurf 账号管理"
    icon="🏄"
    :api="windsurfAPI"
    :can-activate="true"
    token-label="Access Token"
    token-field="access_token"
  />
</template>
<script setup>
import GenericAccountView from './GenericAccountView.vue'
import { windsurfAPI } from '@/api'
</script>
