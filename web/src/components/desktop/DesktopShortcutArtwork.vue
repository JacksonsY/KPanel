<script setup lang="ts">
import { computed } from 'vue'
import FileEntryIcon from '@/components/files/FileEntryIcon.vue'
import { fileEntryIconKind, fileIconPalette } from '@/lib/fileEntryPresentation'
import { useDesktopShellStyle } from '@/stores/desktopShellStyle'

const DIRECTORY_ARTWORK_URL = '/desktop-icons/folder-open-shortcut-kpanel-flat-v1.webp'
const shellStyle = useDesktopShellStyle()
const directoryArtworkURL = computed(() => shellStyle.preference.value === 'modern'
  ? DIRECTORY_ARTWORK_URL
  : `/desktop-icons/${shellStyle.preference.value}/files-v1.svg`)

const props = defineProps<{
  kind: 'file' | 'directory'
  name: string
}>()

const fileColor = computed(() => fileIconPalette[fileEntryIconKind({
  name: props.name, kind: 'file', editable: false, previewable: false,
})][0])
</script>

<template>
  <span
    class="desktop__shortcut-artwork"
    :class="`desktop__shortcut-artwork--${kind}`"
    :style="kind === 'file' ? { '--desktop-file-color': fileColor } : undefined"
    aria-hidden="true"
  >
    <img
      v-if="kind === 'directory'"
      class="desktop__shortcut-directory-image"
      :src="directoryArtworkURL"
      alt=""
      draggable="false"
      width="58"
      height="58"
    />
    <FileEntryIcon
      v-else
      :entry="{ name, kind: 'file' }"
      :size="44"
    />
  </span>
</template>
