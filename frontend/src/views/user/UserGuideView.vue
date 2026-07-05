<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-6">
      <section class="guide-hero overflow-hidden rounded-[28px] border border-slate-200/70 p-6 shadow-sm dark:border-slate-700/80 md:p-8">
        <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_320px]">
          <div>
            <div class="inline-flex items-center gap-2 rounded-full border border-white/60 bg-white/80 px-3 py-1 text-xs font-semibold text-slate-700 shadow-sm backdrop-blur dark:border-white/10 dark:bg-slate-900/60 dark:text-slate-200">
              <Icon name="book" size="sm" />
              <span>{{ copy.badge }}</span>
            </div>

            <h2 class="mt-4 max-w-3xl text-3xl font-bold tracking-tight text-slate-950 dark:text-white md:text-4xl">
              {{ copy.heroTitle }}
            </h2>
            <p class="mt-3 max-w-3xl text-sm leading-7 text-slate-600 dark:text-slate-300 md:text-base">
              {{ copy.heroDescription }}
            </p>

            <div class="mt-6 flex flex-wrap gap-3">
              <router-link
                v-for="action in visibleLinks(copy.primaryActions)"
                :key="action.to"
                :to="action.to"
                :class="[
                  'inline-flex items-center gap-2 rounded-xl px-4 py-2.5 text-sm font-semibold transition-colors',
                  action.emphasis === 'solid'
                    ? 'bg-slate-950 text-white hover:bg-slate-800 dark:bg-white dark:text-slate-950 dark:hover:bg-slate-200'
                    : 'border border-slate-200 bg-white/90 text-slate-700 hover:border-slate-300 hover:bg-white dark:border-slate-700 dark:bg-slate-900/70 dark:text-slate-200 dark:hover:border-slate-500 dark:hover:bg-slate-900'
                ]"
              >
                <Icon :name="action.icon" size="sm" />
                <span>{{ action.label }}</span>
              </router-link>
            </div>

            <div class="mt-6 grid gap-3 sm:grid-cols-3">
              <article
                v-for="fact in copy.heroFacts"
                :key="fact.title"
                class="rounded-2xl border border-white/70 bg-white/80 p-4 shadow-sm backdrop-blur dark:border-white/10 dark:bg-slate-900/70"
              >
                <div class="flex items-center gap-2 text-sm font-semibold text-slate-900 dark:text-white">
                  <span class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200">
                    <Icon :name="fact.icon" size="sm" />
                  </span>
                  <span>{{ fact.title }}</span>
                </div>
                <p class="mt-3 text-sm leading-6 text-slate-600 dark:text-slate-300">
                  {{ fact.body }}
                </p>
              </article>
            </div>
          </div>

          <aside class="rounded-3xl border border-white/70 bg-white/85 p-5 shadow-sm backdrop-blur dark:border-white/10 dark:bg-slate-900/75">
            <div class="flex items-center gap-2 text-sm font-semibold text-slate-900 dark:text-white">
              <span class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300">
                <Icon name="sparkles" size="sm" />
              </span>
              <span>{{ copy.quickStartTitle }}</span>
            </div>

            <ol class="mt-4 space-y-3">
              <li
                v-for="(step, index) in copy.quickStartSteps"
                :key="step"
                class="flex gap-3"
              >
                <span class="mt-0.5 inline-flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full bg-slate-900 text-xs font-bold text-white dark:bg-slate-200 dark:text-slate-950">
                  {{ index + 1 }}
                </span>
                <span class="text-sm leading-6 text-slate-600 dark:text-slate-300">
                  {{ step }}
                </span>
              </li>
            </ol>

            <div class="mt-5 rounded-2xl border border-amber-200 bg-amber-50 p-4 text-sm leading-6 text-amber-900 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-100">
              <div class="flex items-center gap-2 font-semibold">
                <Icon name="infoCircle" size="sm" />
                <span>{{ copy.siteSpecificTitle }}</span>
              </div>
              <p class="mt-2">{{ copy.siteSpecificBody }}</p>
            </div>
          </aside>
        </div>
      </section>

      <div class="rounded-2xl border border-slate-200 bg-white px-4 py-3 dark:border-slate-800 dark:bg-slate-950 xl:hidden">
        <div class="mb-2 text-xs font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-400">
          {{ copy.tocTitle }}
        </div>
        <div class="flex flex-wrap gap-2">
          <a
            v-for="section in copy.sections"
            :key="section.id"
            :href="`#${section.id}`"
            class="rounded-full border border-slate-200 px-3 py-1.5 text-sm text-slate-600 transition-colors hover:border-slate-300 hover:text-slate-900 dark:border-slate-700 dark:text-slate-300 dark:hover:border-slate-500 dark:hover:text-white"
          >
            {{ section.title }}
          </a>
        </div>
      </div>

      <div class="grid gap-6 xl:grid-cols-[minmax(0,1fr)_280px]">
        <main class="space-y-6">
          <section
            v-for="section in copy.sections"
            :id="section.id"
            :key="section.id"
            class="scroll-mt-24 rounded-[28px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-950 md:p-8"
          >
            <div class="flex flex-col gap-4 md:flex-row md:items-start md:justify-between">
              <div class="max-w-3xl">
                <p class="text-xs font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-400">
                  {{ section.kicker }}
                </p>
                <h3 class="mt-2 text-2xl font-bold tracking-tight text-slate-950 dark:text-white">
                  {{ section.title }}
                </h3>
                <p class="mt-3 text-sm leading-7 text-slate-600 dark:text-slate-300 md:text-base">
                  {{ section.summary }}
                </p>
              </div>

              <a
                :href="`#${section.id}`"
                class="inline-flex items-center gap-2 rounded-full border border-slate-200 px-3 py-1.5 text-xs font-semibold text-slate-500 transition-colors hover:border-slate-300 hover:text-slate-900 dark:border-slate-700 dark:text-slate-400 dark:hover:border-slate-500 dark:hover:text-white"
              >
                <Icon name="link" size="xs" />
                <span>#{{ section.id }}</span>
              </a>
            </div>

            <div
              v-if="section.cards?.length"
              class="mt-6 grid gap-4 md:grid-cols-2"
              :class="{ 'xl:grid-cols-3': section.cards.length >= 3 }"
            >
              <article
                v-for="card in section.cards"
                :key="card.title"
                :class="[
                  'rounded-2xl border p-5',
                  toneClass(card.tone)
                ]"
              >
                <div class="flex items-center gap-2 text-sm font-semibold">
                  <span class="inline-flex h-8 w-8 items-center justify-center rounded-xl bg-white/70 text-current dark:bg-black/10">
                    <Icon :name="card.icon" size="sm" />
                  </span>
                  <span>{{ card.title }}</span>
                </div>
                <p class="mt-3 text-sm leading-6 opacity-80">
                  {{ card.body }}
                </p>
              </article>
            </div>

            <ol v-if="section.steps?.length" class="mt-6 space-y-3">
              <li
                v-for="(step, index) in section.steps"
                :key="step"
                class="flex gap-4 rounded-2xl border border-slate-200 bg-slate-50 px-4 py-4 dark:border-slate-800 dark:bg-slate-900/60"
              >
                <span class="inline-flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-slate-950 text-sm font-bold text-white dark:bg-slate-200 dark:text-slate-950">
                  {{ index + 1 }}
                </span>
                <span class="pt-0.5 text-sm leading-6 text-slate-700 dark:text-slate-200">
                  {{ step }}
                </span>
              </li>
            </ol>

            <ul v-if="section.points?.length" class="mt-6 grid gap-3 md:grid-cols-2">
              <li
                v-for="point in section.points"
                :key="point"
                class="rounded-2xl border border-slate-200 bg-white px-4 py-4 text-sm leading-6 text-slate-700 dark:border-slate-800 dark:bg-slate-900/70 dark:text-slate-200"
              >
                <div class="flex gap-3">
                  <span class="mt-1 inline-flex h-2.5 w-2.5 flex-shrink-0 rounded-full bg-slate-950 dark:bg-slate-200"></span>
                  <span>{{ point }}</span>
                </div>
              </li>
            </ul>

            <div
              v-if="section.note"
              class="mt-6 rounded-2xl border border-sky-200 bg-sky-50 px-4 py-4 text-sm leading-6 text-sky-900 dark:border-sky-900/50 dark:bg-sky-950/30 dark:text-sky-100"
            >
              <div class="flex items-center gap-2 font-semibold">
                <Icon name="infoCircle" size="sm" />
                <span>{{ copy.noteTitle }}</span>
              </div>
              <p class="mt-2">{{ section.note }}</p>
            </div>

            <div v-if="section.faqs?.length" class="mt-6 space-y-3">
              <details
                v-for="faq in section.faqs"
                :key="faq.q"
                class="group rounded-2xl border border-slate-200 bg-slate-50 px-4 py-4 dark:border-slate-800 dark:bg-slate-900/60"
              >
                <summary class="flex cursor-pointer list-none items-start justify-between gap-4 text-left text-sm font-semibold text-slate-900 dark:text-white">
                  <span>{{ faq.q }}</span>
                  <span class="mt-0.5 text-slate-400 transition-transform group-open:rotate-45">
                    <Icon name="plus" size="sm" />
                  </span>
                </summary>
                <p class="mt-3 pr-8 text-sm leading-7 text-slate-600 dark:text-slate-300">
                  {{ faq.a }}
                </p>
              </details>
            </div>

            <div v-if="visibleLinks(section.links).length" class="mt-6 flex flex-wrap gap-3">
              <router-link
                v-for="link in visibleLinks(section.links)"
                :key="link.to"
                :to="link.to"
                class="inline-flex items-center gap-2 rounded-xl border border-slate-200 bg-white px-4 py-2.5 text-sm font-semibold text-slate-700 transition-colors hover:border-slate-300 hover:bg-slate-50 hover:text-slate-900 dark:border-slate-700 dark:bg-slate-900 dark:text-slate-200 dark:hover:border-slate-500 dark:hover:bg-slate-800 dark:hover:text-white"
              >
                <Icon :name="link.icon" size="sm" />
                <span>{{ link.label }}</span>
              </router-link>
            </div>
          </section>
        </main>

        <aside class="hidden xl:block">
          <div class="sticky top-24 rounded-[28px] border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-950">
            <div class="text-xs font-semibold uppercase tracking-[0.18em] text-slate-500 dark:text-slate-400">
              {{ copy.tocTitle }}
            </div>
            <nav class="mt-4 space-y-1">
              <a
                v-for="section in copy.sections"
                :key="section.id"
                :href="`#${section.id}`"
                class="block rounded-xl px-3 py-2 text-sm text-slate-600 transition-colors hover:bg-slate-100 hover:text-slate-950 dark:text-slate-300 dark:hover:bg-slate-900 dark:hover:text-white"
              >
                {{ section.title }}
              </a>
            </nav>

            <div class="mt-6 rounded-2xl border border-slate-200 bg-slate-50 p-4 dark:border-slate-800 dark:bg-slate-900/60">
              <div class="flex items-center gap-2 text-sm font-semibold text-slate-900 dark:text-white">
                <Icon name="questionCircle" size="sm" />
                <span>{{ copy.needHelpTitle }}</span>
              </div>
              <p class="mt-2 text-sm leading-6 text-slate-600 dark:text-slate-300">
                {{ copy.needHelpBody }}
              </p>
            </div>
          </div>
        </aside>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore } from '@/stores'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'

type GuideIcon =
  | 'book'
  | 'sparkles'
  | 'creditCard'
  | 'key'
  | 'globe'
  | 'chartBar'
  | 'gift'
  | 'terminal'
  | 'shield'
  | 'calculator'
  | 'document'
  | 'badge'
  | 'dollar'
  | 'infoCircle'
  | 'questionCircle'
  | 'plus'
  | 'link'

type LinkFeature = 'always' | 'payment' | 'availableChannels' | 'usage' | 'subscriptions' | 'orders' | 'redeem' | 'affiliate'
type CardTone = 'neutral' | 'accent' | 'success' | 'warn'

interface GuideLink {
  label: string
  to: string
  icon: GuideIcon
  feature?: LinkFeature
  emphasis?: 'solid' | 'outline'
}

interface GuideCard {
  title: string
  body: string
  icon: GuideIcon
  tone?: CardTone
}

interface GuideFaq {
  q: string
  a: string
}

interface GuideSection {
  id: string
  kicker: string
  title: string
  summary: string
  cards?: GuideCard[]
  steps?: string[]
  points?: string[]
  faqs?: GuideFaq[]
  note?: string
  links?: GuideLink[]
}

interface GuideCopy {
  badge: string
  heroTitle: string
  heroDescription: string
  heroFacts: GuideCard[]
  primaryActions: GuideLink[]
  quickStartTitle: string
  quickStartSteps: string[]
  siteSpecificTitle: string
  siteSpecificBody: string
  tocTitle: string
  needHelpTitle: string
  needHelpBody: string
  noteTitle: string
  sections: GuideSection[]
}

const zhCopy: GuideCopy = {
  badge: '用户使用文档',
  heroTitle: '从购买到接入，照着这一页走就够了',
  heroDescription:
    '这页只讲普通用户怎么用：怎么买、买完后怎么创建 API Key、怎么选分组、怎么查看用量。部署、支付配置和后台管理不在这里。',
  heroFacts: [
    {
      title: '先分清购买方式',
      body: '余额充值适合按量使用，订阅套餐适合长期、高频且更想锁定成本的场景。',
      icon: 'creditCard',
      tone: 'neutral',
    },
    {
      title: 'Key 只是入口',
      body: '买完以后还需要创建 API Key，并给它绑定正确分组，才能稳定接入客户端。',
      icon: 'key',
      tone: 'accent',
    },
    {
      title: '分组决定结果',
      body: '分组会影响平台、模型和可见倍率。同一个 Key 换组后，效果和扣费都可能变化。',
      icon: 'globe',
      tone: 'success',
    },
  ],
  primaryActions: [
    { label: '创建 API Key', to: '/keys', icon: 'key', emphasis: 'solid' },
    { label: '去充值/订阅', to: '/purchase', icon: 'creditCard', feature: 'payment', emphasis: 'outline' },
    { label: '查看可用渠道', to: '/available-channels', icon: 'globe', feature: 'availableChannels', emphasis: 'outline' },
    { label: '查看用量', to: '/usage', icon: 'chartBar', feature: 'usage', emphasis: 'outline' },
  ],
  quickStartTitle: '推荐的新手顺序',
  quickStartSteps: [
    '先看购买页，确认自己更适合充值还是订阅套餐。',
    '支付完成后进入 API Keys，创建一个新的 Key。',
    '给这个 Key 绑定正确分组，再复制接入地址。',
    '点击“使用 Key”，直接复制客户端配置或环境变量。',
    '首次调用成功后，再去使用记录核对是否正常计费。',
  ],
  siteSpecificTitle: '站点差异说明',
  siteSpecificBody:
    '你看到的菜单、分组、模型和支付方式可能因站点配置不同而变化。如果某个入口你看不到，通常表示该功能没有开启，或当前账号暂时不可用。',
  tocTitle: '本页目录',
  needHelpTitle: '看完还不确定？',
  needHelpBody:
    '先回到 API Keys 和可用渠道页面对照检查。用户侧最常见的问题，基本都能在“分组没选对”或“模型不在当前分组支持范围内”这两处定位。',
  noteTitle: '补充说明',
  sections: [
    {
      id: 'billing',
      kicker: '01 / 购买方式',
      title: '先分清余额充值和订阅套餐',
      summary:
        '购买页通常同时提供充值和订阅两个入口。先确认你是按量使用，还是长期固定使用，再决定怎么买。',
      cards: [
        {
          title: '余额充值',
          body: '先充值到账户余额，后续按实际调用扣费。适合模型切换多、使用量不固定、想先低成本试用的场景。',
          icon: 'dollar',
          tone: 'neutral',
        },
        {
          title: '订阅套餐',
          body: '按套餐价格购买，通常会看到有效期和日/周/月额度。适合长期、高频使用某个平台或某类能力。',
          icon: 'creditCard',
          tone: 'accent',
        },
        {
          title: '到账预览',
          body: '如果站点设置了充值赠送，购买页会直接显示预计到账余额。实际到账金额以购买页展示为准，不需要自己换算。',
          icon: 'gift',
          tone: 'success',
        },
      ],
      links: [
        { label: '去充值/订阅', to: '/purchase', icon: 'creditCard', feature: 'payment' },
        { label: '查看我的订单', to: '/orders', icon: 'document', feature: 'orders' },
      ],
    },
    {
      id: 'after-purchase',
      kicker: '02 / 购买后接入',
      title: '买完以后，不是直接调用，而是先配置 API Key',
      summary:
        '要真正开始用，至少要完成创建 Key、绑定分组、复制接入地址、复制客户端配置这四步。',
      steps: [
        '进入 API Keys 页面，创建一个新的 API Key，并给它起一个你看得懂的名字。',
        '给这个 Key 绑定正确分组。没有分组时，平台、模型和对应客户端配置通常都无法确定。',
        '复制 API Keys 页面顶部展示的接入地址，直接用页面给出的地址，不要自己手动猜测或拼接。',
        '点击单个 Key 的“使用 Key”，直接复制弹窗里的环境变量或配置文件。',
      ],
      cards: [
        {
          title: '最稳的做法',
          body: '先绑分组，再点“使用 Key”。弹窗会根据平台自动生成 Claude Code、Codex CLI、Gemini CLI、OpenCode 等接入示例。',
          icon: 'terminal',
          tone: 'accent',
        },
        {
          title: '避免踩坑',
          body: '如果你直接把同一个 Key 反复切换分组，已经在运行的客户端也可能一起受影响。想同时跑多个平台时，推荐直接创建多个 Key。',
          icon: 'shield',
          tone: 'warn',
        },
      ],
      links: [
        { label: '去 API Keys', to: '/keys', icon: 'key' },
        { label: '查看用量', to: '/usage', icon: 'chartBar', feature: 'usage' },
      ],
    },
    {
      id: 'groups',
      kicker: '03 / 分组怎么选',
      title: '选分组时，先看平台，再看倍率，再看模型',
      summary:
        '分组决定了一个 Key 走哪个平台、支持哪些模型，以及你在用户侧看到的可见倍率。不要只看分组名字。',
      cards: [
        {
          title: '先看平台',
          body: 'Claude / OpenAI / Gemini 这类平台不能混着猜。分组绑定的平台不同，接入方式也会不同。',
          icon: 'globe',
          tone: 'neutral',
        },
        {
          title: '再看倍率',
          body: '有些分组会区分余额倍率和套餐倍率；如果你有专属倍率，界面展示的会是对你当前生效的可见倍率。',
          icon: 'calculator',
          tone: 'accent',
        },
        {
          title: '最后看模型',
          body: '如果站点开启了“可用渠道”，优先在那里确认这个分组到底支持哪些模型，而不是只凭名称判断。',
          icon: 'document',
          tone: 'success',
        },
        {
          title: '套餐用户额外看权限',
          body: '有些分组只有套餐生效后才会出现，或者套餐倍率与余额倍率不同。买了套餐后，先回到 API Keys 检查你能绑定哪些分组。',
          icon: 'badge',
          tone: 'warn',
        },
      ],
      note:
        '一个 Key 一次只能绑定一个分组。想同时跑多个平台或不同成本策略时，最简单的做法是直接拆成多个 Key，而不是反复切换同一个 Key。',
      links: [
        { label: '查看可用渠道', to: '/available-channels', icon: 'globe', feature: 'availableChannels' },
        { label: '管理我的 Key', to: '/keys', icon: 'key' },
      ],
    },
    {
      id: 'pages',
      kicker: '04 / 常用页面',
      title: '不知道去哪里看，就按这几个页面分工走',
      summary:
        '用户端已经把“买、配、查、对账”拆成了不同页面。出问题时，按页面分工回看，定位会更快。',
      points: [
        'API Keys：创建 Key、绑定分组、复制接入地址、打开“使用 Key”弹窗。',
        'Usage / 使用记录：查看请求数、Token、实际花费，并按 Key 或时间筛选。',
        'My Subscriptions / 我的订阅：查看套餐是否生效、到期时间和额度使用进度。',
        'My Orders / 我的订单：核对充值单、套餐单和支付状态。',
        'Redeem / Affiliate：如果站点开启了兑换码或邀请返利，对应入口会出现在这里。',
      ],
      links: [
        { label: '去 API Keys', to: '/keys', icon: 'key' },
        { label: '去使用记录', to: '/usage', icon: 'chartBar', feature: 'usage' },
        { label: '去我的订阅', to: '/subscriptions', icon: 'creditCard', feature: 'subscriptions' },
        { label: '去我的订单', to: '/orders', icon: 'document', feature: 'orders' },
        { label: '去兑换', to: '/redeem', icon: 'gift', feature: 'redeem' },
      ],
    },
    {
      id: 'faq',
      kicker: '05 / 常见问题',
      title: '第一次使用最容易卡住的地方',
      summary:
        '下面这些问题最常见，也最容易影响首次接入。先对照检查，再决定是不是需要联系站点管理员。',
      faqs: [
        {
          q: '我已经充值/购买了，为什么还是不能用？',
          a: '先检查订单是否支付成功、是否已经创建 API Key、Key 是否已绑定正确分组，以及客户端里填入的接入地址是否来自 API Keys 页面。',
        },
        {
          q: '买了套餐，为什么看不到预期分组？',
          a: '先去“我的订阅”确认套餐是否已生效，再回到 API Keys 或可用渠道页面核对分组。分组名称可能和套餐名称不完全一致。',
        },
        {
          q: '同一个 Key 换组后，为什么效果不一样？',
          a: '因为分组会影响平台、可用模型和计费倍率。换组后，客户端配置虽然可能不变，但实际可用能力和扣费结果会变化。',
        },
        {
          q: '每个 Key 能看到上游倍率、上游账号或内部路由吗？',
          a: '通常不能。用户侧能看到的是自己可绑定分组的可见倍率、能访问的模型和自己的用量；内部路由和上游账号信息一般不对普通用户展示。',
        },
        {
          q: '充值和套餐应该怎么选？',
          a: '用量不稳定、会频繁切平台或模型时，先用充值。长期固定使用某个平台、希望成本更稳定时，再考虑订阅套餐。',
        },
      ],
      links: [
        { label: '查看用量', to: '/usage', icon: 'chartBar', feature: 'usage' },
        { label: '查看我的订阅', to: '/subscriptions', icon: 'creditCard', feature: 'subscriptions' },
      ],
    },
  ],
}

const enCopy: GuideCopy = {
  badge: 'User Guide',
  heroTitle: 'Purchase, configure, and start calling the API from one page',
  heroDescription:
    'This page is only for end users: how to buy, how to create API keys, how to choose groups, and how to verify usage after you start using the service.',
  heroFacts: [
    {
      title: 'Choose the billing mode first',
      body: 'Balance top-up is better for variable usage. Subscription plans are better for stable, high-frequency usage.',
      icon: 'creditCard',
      tone: 'neutral',
    },
    {
      title: 'Keys are only the entry point',
      body: 'After you buy, you still need to create an API key and bind it to the correct group before the client config is stable.',
      icon: 'key',
      tone: 'accent',
    },
    {
      title: 'Groups change the outcome',
      body: 'A group affects the platform, available models, and visible multiplier. Switching a key to another group may change cost and behavior.',
      icon: 'globe',
      tone: 'success',
    },
  ],
  primaryActions: [
    { label: 'Create API Key', to: '/keys', icon: 'key', emphasis: 'solid' },
    { label: 'Recharge / Subscribe', to: '/purchase', icon: 'creditCard', feature: 'payment', emphasis: 'outline' },
    { label: 'Available Channels', to: '/available-channels', icon: 'globe', feature: 'availableChannels', emphasis: 'outline' },
    { label: 'Usage Records', to: '/usage', icon: 'chartBar', feature: 'usage', emphasis: 'outline' },
  ],
  quickStartTitle: 'Recommended first-time flow',
  quickStartSteps: [
    'Decide whether balance top-up or a subscription plan fits you better.',
    'After payment, go to API Keys and create a new key.',
    'Bind that key to the correct group, then copy the endpoint shown on the page.',
    'Open "Use Key" and copy the generated client config or environment variables.',
    'After the first successful request, verify billing in Usage Records.',
  ],
  siteSpecificTitle: 'Site-specific differences',
  siteSpecificBody:
    'Menus, groups, models, and payment methods may differ by deployment. If you do not see an entry, that feature is likely disabled for the site or unavailable for your account.',
  tocTitle: 'On this page',
  needHelpTitle: 'Still unsure?',
  needHelpBody:
    'Start with API Keys and Available Channels. Most first-time issues come from binding the wrong group or choosing a model that the current group does not support.',
  noteTitle: 'Note',
  sections: [
    {
      id: 'billing',
      kicker: '01 / Billing',
      title: 'Understand balance top-up vs subscription plans first',
      summary:
        'The purchase page usually exposes both options. Decide whether you need flexible pay-as-you-go usage or a plan with quota and a fixed pricing model.',
      cards: [
        {
          title: 'Balance top-up',
          body: 'Top up your balance first, then pay based on actual usage. This is better when your usage varies or you switch between models often.',
          icon: 'dollar',
          tone: 'neutral',
        },
        {
          title: 'Subscription plans',
          body: 'Plans usually show price, validity, and daily/weekly/monthly quota. This is better for long-term, stable usage.',
          icon: 'creditCard',
          tone: 'accent',
        },
        {
          title: 'Credited amount preview',
          body: 'If the site offers a recharge bonus, the purchase page will show the expected credited amount directly. Use the page value, not manual math.',
          icon: 'gift',
          tone: 'success',
        },
      ],
      links: [
        { label: 'Go to Purchase', to: '/purchase', icon: 'creditCard', feature: 'payment' },
        { label: 'My Orders', to: '/orders', icon: 'document', feature: 'orders' },
      ],
    },
    {
      id: 'after-purchase',
      kicker: '02 / After Purchase',
      title: 'After paying, configure the API key before you call the service',
      summary:
        'The minimum path is: create a key, bind a group, copy the endpoint, and use the generated client config.',
      steps: [
        'Open API Keys and create a new key with a name that you can recognize later.',
        'Bind the key to the correct group. Without a group, the platform and model path are usually undefined.',
        'Copy the endpoint shown at the top of API Keys instead of guessing or building it manually.',
        'Open "Use Key" and copy the client config or environment variables generated for that platform.',
      ],
      cards: [
        {
          title: 'Most reliable path',
          body: 'Bind the group first, then open "Use Key". The modal generates platform-specific examples for tools such as Claude Code, Codex CLI, Gemini CLI, and OpenCode.',
          icon: 'terminal',
          tone: 'accent',
        },
        {
          title: 'Avoid this pitfall',
          body: 'If you keep reusing one key and switching its group, running clients may be affected too. When you use multiple platforms, create multiple keys.',
          icon: 'shield',
          tone: 'warn',
        },
      ],
      links: [
        { label: 'Open API Keys', to: '/keys', icon: 'key' },
        { label: 'Open Usage', to: '/usage', icon: 'chartBar', feature: 'usage' },
      ],
    },
    {
      id: 'groups',
      kicker: '03 / Groups',
      title: 'Choose groups by platform first, then multiplier, then models',
      summary:
        'Groups decide which platform a key uses, which models it can access, and which visible multiplier applies to your usage.',
      cards: [
        {
          title: 'Check the platform first',
          body: 'Claude, OpenAI, Gemini, and custom platforms are not interchangeable. The bound platform changes the client setup path.',
          icon: 'globe',
          tone: 'neutral',
        },
        {
          title: 'Then check multipliers',
          body: 'Some groups have separate balance and subscription multipliers. If you have a user-specific override, the UI shows the multiplier that is visible to you.',
          icon: 'calculator',
          tone: 'accent',
        },
        {
          title: 'Then check supported models',
          body: 'If Available Channels is enabled, use it to confirm the supported models instead of relying on the group name.',
          icon: 'document',
          tone: 'success',
        },
        {
          title: 'Plan users should also check access',
          body: 'Some groups only appear after a plan becomes active, or apply a different multiplier for plan-based usage.',
          icon: 'badge',
          tone: 'warn',
        },
      ],
      note:
        'One key can only bind one group at a time. If you want separate platforms or cost strategies, split them into multiple keys.',
      links: [
        { label: 'Available Channels', to: '/available-channels', icon: 'globe', feature: 'availableChannels' },
        { label: 'Manage Keys', to: '/keys', icon: 'key' },
      ],
    },
    {
      id: 'pages',
      kicker: '04 / Core Pages',
      title: 'Use the built-in pages by responsibility',
      summary:
        'The user console already separates buying, configuring, checking usage, and reconciling orders into different pages. Use them as intended.',
      points: [
        'API Keys: create keys, bind groups, copy endpoints, and open the "Use Key" modal.',
        'Usage Records: inspect request count, tokens, actual cost, and filter by key or time range.',
        'My Subscriptions: verify whether a plan is active, when it expires, and how much quota is left.',
        'My Orders: review recharge orders, plan orders, and payment status.',
        'Redeem / Affiliate: these appear when the site enables redeem codes or referral rebates.',
      ],
      links: [
        { label: 'API Keys', to: '/keys', icon: 'key' },
        { label: 'Usage', to: '/usage', icon: 'chartBar', feature: 'usage' },
        { label: 'My Subscriptions', to: '/subscriptions', icon: 'creditCard', feature: 'subscriptions' },
        { label: 'My Orders', to: '/orders', icon: 'document', feature: 'orders' },
        { label: 'Redeem', to: '/redeem', icon: 'gift', feature: 'redeem' },
      ],
    },
    {
      id: 'faq',
      kicker: '05 / FAQ',
      title: 'The most common first-time issues',
      summary:
        'Most onboarding issues can be diagnosed from the user console without touching admin settings. Check these first.',
      faqs: [
        {
          q: 'I paid, but why can’t I use it yet?',
          a: 'Check that the order is paid successfully, that you created an API key, that the key is bound to the correct group, and that your client endpoint came from the API Keys page.',
        },
        {
          q: 'I bought a plan, but why can’t I see the expected group?',
          a: 'Confirm that the plan is active in My Subscriptions, then check API Keys or Available Channels again. Group names may not match the plan name exactly.',
        },
        {
          q: 'Why does the same key behave differently after I switch the group?',
          a: 'Because groups affect the platform, the available models, and the multiplier. The client config may look similar while the actual routed capability changes.',
        },
        {
          q: 'Can each key show upstream multipliers, upstream accounts, or internal routing?',
          a: 'Usually no. Users normally only see the groups they can bind, the visible multipliers for those groups, supported models, and their own usage and orders.',
        },
        {
          q: 'How should I choose between recharge and a plan?',
          a: 'Start with recharge if your usage is irregular or you switch platforms frequently. Use a plan when your usage is stable and you want a more predictable cost structure.',
        },
      ],
      links: [
        { label: 'Usage Records', to: '/usage', icon: 'chartBar', feature: 'usage' },
        { label: 'My Subscriptions', to: '/subscriptions', icon: 'creditCard', feature: 'subscriptions' },
      ],
    },
  ],
}

const { locale } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const copy = computed<GuideCopy>(() => (
  locale.value.toLowerCase().startsWith('zh') ? zhCopy : enCopy
))

const linkFlags = computed<Record<LinkFeature, boolean>>(() => ({
  always: true,
  payment: appStore.cachedPublicSettings?.payment_enabled === true && !authStore.isSimpleMode,
  availableChannels: appStore.cachedPublicSettings?.available_channels_enabled === true && !authStore.isSimpleMode,
  usage: !authStore.isSimpleMode,
  subscriptions: !authStore.isSimpleMode,
  orders: appStore.cachedPublicSettings?.payment_enabled === true && !authStore.isSimpleMode,
  redeem: !authStore.isSimpleMode,
  affiliate: appStore.cachedPublicSettings?.affiliate_enabled === true && !authStore.isSimpleMode,
}))

function visibleLinks(links?: GuideLink[]): GuideLink[] {
  if (!links?.length) return []
  return links.filter((link) => linkFlags.value[link.feature ?? 'always'])
}

function toneClass(tone: CardTone = 'neutral'): string {
  switch (tone) {
    case 'accent':
      return 'border-sky-200 bg-sky-50 text-sky-950 dark:border-sky-900/50 dark:bg-sky-950/30 dark:text-sky-100'
    case 'success':
      return 'border-emerald-200 bg-emerald-50 text-emerald-950 dark:border-emerald-900/50 dark:bg-emerald-950/30 dark:text-emerald-100'
    case 'warn':
      return 'border-amber-200 bg-amber-50 text-amber-950 dark:border-amber-900/50 dark:bg-amber-950/30 dark:text-amber-100'
    default:
      return 'border-slate-200 bg-slate-50 text-slate-950 dark:border-slate-800 dark:bg-slate-900/60 dark:text-slate-100'
  }
}
</script>

<style scoped>
.guide-hero {
  background:
    radial-gradient(circle at top left, rgba(56, 189, 248, 0.26), transparent 36%),
    radial-gradient(circle at top right, rgba(14, 165, 233, 0.18), transparent 28%),
    linear-gradient(135deg, #f8fbff 0%, #f7fafc 42%, #eef6ff 100%);
}

:global(.dark) .guide-hero {
  background:
    radial-gradient(circle at top left, rgba(56, 189, 248, 0.18), transparent 32%),
    radial-gradient(circle at top right, rgba(59, 130, 246, 0.12), transparent 24%),
    linear-gradient(135deg, #0f172a 0%, #111827 40%, #0b1220 100%);
}
</style>
