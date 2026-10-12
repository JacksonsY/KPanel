<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import { ExternalLink, LayoutDashboard, Search, ShieldCheck } from '@lucide/vue'
import { usePhraseCatalog } from '@/i18n/phrase'
import PageHeader from '@/components/common/PageHeader.vue'
import { useI18n } from '@/i18n'

usePhraseCatalog((locale) => locale === 'en-US'
  ? import('@/i18n/pages/SettingsGuidesView/en-US').then((module) => module.default)
  : import('@/i18n/pages/SettingsGuidesView/zh-TW').then((module) => module.default))

const i18n = useI18n()
const isEnglish = computed(() => i18n.locale.value === 'en-US')

const video = computed(() => isEnglish.value
  ? {
      href: 'https://www.youtube.com/watch?v=HLTtsALHlpw&hl=en',
      poster: '/tutorials/kpanel-overview-en.webp',
      title: i18n.t('tutorials.video.titleEn'),
      description: i18n.t('tutorials.video.descriptionEn'),
      alt: i18n.t('tutorials.video.altEn'),
      platform: 'YouTube',
    }
  : {
      href: 'https://www.bilibili.com/video/BV1e7ar69EfD/',
      poster: '/tutorials/kpanel-overview-zh.webp',
      title: i18n.t('tutorials.video.titleZh'),
      description: i18n.t('tutorials.video.descriptionZh'),
      alt: i18n.t('tutorials.video.altZh'),
      platform: 'Bilibili',
    })

const quickSteps = computed(() => [
  {
    icon: LayoutDashboard,
    title: i18n.t('tutorials.quick.modeTitle'),
    body: i18n.t('tutorials.quick.modeBody'),
  },
  {
    icon: Search,
    title: i18n.t('tutorials.quick.featuresTitle'),
    body: i18n.t('tutorials.quick.featuresBody'),
  },
  {
    icon: ShieldCheck,
    title: i18n.t('tutorials.quick.backupTitle'),
    body: i18n.t('tutorials.quick.backupBody'),
  },
])
</script>

<template>
  <div class="page page--narrow guide-page">
    <PageHeader :title="i18n.t('route.settingsGuide')" :description="i18n.t('tutorials.description')" />

    <header class="guide-intro panel-card">
      <div>
        <span class="guide-eyebrow">{{ i18n.t('tutorials.eyebrow') }}</span>
        <h1>{{ i18n.t('tutorials.title') }}</h1>
        <p>{{ i18n.t('tutorials.description') }}</p>
      </div>
      <RouterLink class="button button--secondary guide-back" to="/settings">
        <span aria-hidden="true">←</span>
        {{ i18n.t('tutorials.back') }}
      </RouterLink>
    </header>

    <section class="guide-section" aria-labelledby="guide-video-heading">
      <div class="guide-section__heading">
        <div>
          <h2 id="guide-video-heading">{{ i18n.t('tutorials.video.heading') }}</h2>
          <p>{{ i18n.t('tutorials.video.summary') }}</p>
        </div>
        <span class="guide-platform">{{ video.platform }}</span>
      </div>

      <a
        class="guide-video panel-card"
        data-testid="tutorial-video-link"
        :href="video.href"
        target="_blank"
        rel="noopener noreferrer"
        referrerpolicy="no-referrer"
        :aria-label="i18n.t('tutorials.video.open', { platform: video.platform, title: video.title })"
      >
        <span class="guide-video__poster">
          <img
            :src="video.poster"
            :alt="video.alt"
            width="1280"
            height="720"
            loading="lazy"
            decoding="async"
          />
          <span class="guide-video__play" aria-hidden="true">
            <span>▶</span>
          </span>
        </span>
        <span class="guide-video__content">
          <span class="guide-video__meta">{{ video.platform }} · KPanel</span>
          <strong>{{ video.title }}</strong>
          <span class="guide-video__description">{{ video.description }}</span>
          <span class="guide-video__action">
            {{ i18n.t('tutorials.video.watch', { platform: video.platform }) }}
            <ExternalLink :size="16" aria-hidden="true" />
          </span>
        </span>
      </a>

      <p class="guide-video-note">{{ i18n.t('tutorials.video.externalNote') }}</p>
      <p v-if="isEnglish" class="guide-video-note">{{ i18n.t('tutorials.video.audioNote') }}</p>
    </section>

    <section class="guide-section" aria-labelledby="guide-quick-heading">
      <div class="guide-section__heading">
        <div>
          <h2 id="guide-quick-heading">{{ i18n.t('tutorials.quick.heading') }}</h2>
          <p>{{ i18n.t('tutorials.quick.summary') }}</p>
        </div>
      </div>
      <ol class="guide-steps">
        <li v-for="step in quickSteps" :key="step.title" class="guide-step panel-card">
          <span class="guide-step__icon" aria-hidden="true">
            <component :is="step.icon" :size="21" />
          </span>
          <span class="guide-step__content">
            <strong>{{ step.title }}</strong>
            <span>{{ step.body }}</span>
          </span>
        </li>
      </ol>
    </section>
  </div>
</template>

<style scoped>
.guide-page {
  display: grid;
  gap: 22px;
}

.guide-intro {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  padding: 22px;
}

.guide-eyebrow {
  color: var(--brand-strong);
  font-size: 0.8125rem;
  font-weight: 600;
}

.guide-intro h1 {
  margin: 6px 0 8px;
  color: var(--text);
  font-size: clamp(1.375rem, 3vw, 1.75rem);
  line-height: 1.25;
}

.guide-intro p,
.guide-section__heading p {
  margin: 0;
  color: var(--text-soft);
  font-size: 0.875rem;
  line-height: 1.55;
}

.guide-back {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 44px;
  text-decoration: none;
}

.guide-section {
  display: grid;
  gap: 12px;
}

.guide-section__heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 0 2px;
}

.guide-section__heading h2 {
  margin: 0 0 5px;
  color: var(--text);
  font-size: 1.125rem;
  line-height: 1.3;
}

.guide-platform {
  flex: 0 0 auto;
  padding: 5px 10px;
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  color: var(--text-soft);
  font-size: 0.8125rem;
  font-weight: 600;
}

.guide-video {
  display: grid;
  grid-template-columns: minmax(0, 1.25fr) minmax(250px, 0.75fr);
  overflow: hidden;
  border: 1px solid var(--border);
  color: inherit;
  text-decoration: none;
  transition:
    border-color var(--motion-duration-fast) var(--motion-ease-fade),
    box-shadow var(--motion-duration-fast) var(--motion-ease-fade),
    transform var(--motion-duration-fast) var(--motion-ease-standard);
}

.guide-video:hover {
  border-color: var(--border-strong);
  box-shadow: var(--shadow-sm);
  transform: translateY(-1px);
}

.guide-video:focus-visible,
.guide-back:focus-visible {
  outline: 3px solid var(--brand);
  outline-offset: 3px;
}

.guide-video__poster {
  position: relative;
  display: block;
  overflow: hidden;
  aspect-ratio: 16 / 9;
  background: var(--surface-subtle);
}

.guide-video__poster img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.guide-video__play {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  background: color-mix(in srgb, #0b1220 15%, transparent);
  color: white;
  transition: background var(--motion-duration-fast) var(--motion-ease-fade);
}

.guide-video:hover .guide-video__play {
  background: color-mix(in srgb, #0b1220 26%, transparent);
}

.guide-video__play span {
  display: grid;
  width: 58px;
  height: 58px;
  place-items: center;
  padding-left: 3px;
  border: 1px solid rgb(255 255 255 / 72%);
  border-radius: 50%;
  background: rgb(15 23 42 / 74%);
  font-size: 1.25rem;
}

.guide-video__content {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 12px;
  padding: clamp(18px, 3vw, 26px);
}

.guide-video__meta {
  color: var(--brand-strong);
  font-size: 0.8125rem;
  font-weight: 600;
}

.guide-video__content strong {
  color: var(--text);
  font-size: 1.125rem;
  line-height: 1.4;
}

.guide-video__description {
  color: var(--text-soft);
  font-size: 0.875rem;
  line-height: 1.55;
}

.guide-video__action {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  margin-top: auto;
  color: var(--brand-strong);
  font-size: 0.875rem;
  font-weight: 600;
}

.guide-video-note {
  margin: 0;
  color: var(--muted);
  font-size: 0.8125rem;
  line-height: 1.55;
}

.guide-steps {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin: 0;
  padding: 0;
  list-style: none;
}

.guide-step {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  min-width: 0;
  padding: 18px;
}

.guide-step__icon {
  flex: 0 0 40px;
  display: grid;
  width: 40px;
  height: 40px;
  place-items: center;
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: var(--surface-subtle);
  color: var(--brand-strong);
}

.guide-step__content {
  display: grid;
  gap: 7px;
  min-width: 0;
}

.guide-step__content strong {
  color: var(--text);
  font-size: 0.9375rem;
  line-height: 1.4;
}

.guide-step__content > span {
  color: var(--text-soft);
  font-size: 0.875rem;
  line-height: 1.55;
}

@media (max-width: 760px) {
  .guide-video {
    grid-template-columns: 1fr;
  }

  .guide-steps {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 540px) {
  .guide-intro {
    flex-direction: column;
    padding: 18px;
  }

  .guide-back {
    width: 100%;
  }

  .guide-video__content {
    gap: 9px;
    padding: 17px;
  }

  .guide-step {
    padding: 16px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .guide-video,
  .guide-video__play {
    transition: none;
  }
}
</style>
