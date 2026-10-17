<script setup lang="ts">
import { computed } from 'vue'
import { type BackupModule, type BackupReviewResult } from '@/lib/backup'
import { phraseCatalogVersion, translatePhrase } from '@/i18n/phrase'
const props = defineProps<{ result?: BackupReviewResult; modules: BackupModule[] }>()
function phrase(text: string) { phraseCatalogVersion.value; return translatePhrase(text) }
const reviews = computed(() => props.result?.review?.modules.filter(m => props.modules.includes(m.module) && (m.origin !== 'local' || m.unreviewed)) || [])
</script>

<template>
  <section class="import-review" aria-labelledby="import-review-title">
    <h4 id="import-review-title">{{ phrase('外来备份的宿主机配置') }}</h4>
    <p>{{ phrase('无法验证为本机导出的备份。确认恢复即同意下面的目录覆盖、容器权限和网络配置；文件属主与特殊权限也会原样恢复。') }}</p>
    <p v-if="!result?.available || reviews.some(m => m.unreviewed)">{{ phrase('当前 Agent 或旧导入记录无法提供完整配置，请更新 Agent 并重新导入后核对；继续恢复表示你信任该备份的全部配置。') }}</p>
    <article v-for="review in reviews" :key="review.module">
      <h5>{{ review.module }}</h5>
      <p>{{ phrase('含 set-id 权限的文件：') }} {{ review.setIdFiles || 0 }}</p>
      <ul v-if="review.roots?.length">
        <li v-for="root in review.roots" :key="root.path">
          {{ phrase('覆盖目录：') }} <code>{{ root.path }}</code>
          <strong v-if="root.systemPath"> · {{ phrase('系统路径') }}</strong>
          <span v-for="user in root.usedBy" :key="user.container"> · {{ user.container }} ({{ user.readOnly ? 'ro' : 'rw' }})</span>
        </li>
      </ul>
      <details v-for="container in review.containers" :key="container.name" open>
        <summary><code>{{ container.name }}</code> · {{ container.image }}</summary>
        <p>ImageID: <code>{{ container.imageId }}</code></p>
        <p v-if="container.imageRef">ImageRef: <code>{{ container.imageRef }}</code></p>
        <p>{{ phrase('替换同名容器：') }} {{ container.replaces ? phrase('是') : phrase('否') }} · privileged={{ !!container.privileged }}</p>
        <p v-if="container.hostNamespaces?.length">host namespaces: {{ container.hostNamespaces.join(', ') }}</p>
        <p v-if="container.capAdd?.length">CapAdd: {{ container.capAdd.join(', ') }}</p>
        <p v-if="container.devices?.length">Devices: {{ container.devices.join(', ') }}</p>
        <p v-if="container.securityOpt?.length">SecurityOpt: {{ container.securityOpt.join(', ') }}</p>
        <ul v-if="container.binds?.length">
          <li v-for="(bind, index) in container.binds" :key="index">
            <code>{{ bind.source }} → {{ bind.destination }}</code> ({{ bind.readOnly ? 'ro' : 'rw' }})
            <strong v-if="bind.live"> · {{ phrase('直接使用宿主机现有目录') }}</strong>
          </li>
        </ul>
        <p v-if="container.moreItems">{{ phrase('还有未展开的配置项：') }} {{ container.moreItems }}</p>
        <details v-if="container.hostConfig"><summary>HostConfig</summary><pre data-i18n-ignore>{{ JSON.stringify(container.hostConfig, null, 2) }}</pre></details>
      </details>
      <details v-for="network in review.networks" :key="network.name" open>
        <summary>{{ network.name }} · {{ network.driver }}</summary>
        <p>Internal={{ !!network.internal }}</p>
        <p v-for="subnet in network.subnets" :key="subnet"><code>{{ subnet }}</code></p>
        <p v-for="option in network.options" :key="option"><code>{{ option }}</code></p>
        <details v-if="network.config"><summary>NetworkConfig</summary><pre data-i18n-ignore>{{ JSON.stringify(network.config, null, 2) }}</pre></details>
      </details>
    </article>
  </section>
</template>

<style scoped>
.import-review { padding: 14px; border: 1px solid var(--color-border); border-radius: 10px; font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
h4, h5 { margin: 0 0 8px; font-size: 14px; }
p { margin: 8px 0; }
article, details { margin-top: 12px; }
summary { cursor: pointer; }
ul { margin: 8px 0; padding-left: 20px; }
code { font-size: 13px; white-space: normal; }
pre { white-space: pre-wrap; font-size: 13px; overflow-wrap: anywhere; }
</style>
