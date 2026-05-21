import { h, markRaw, onMounted, ref, type Component } from 'vue';
import type { Meta, StoryObj } from '@storybook/vue3';
import { buildGeneratedScreen, type GeneratedScreenView } from './services/generationRenderService';
import type { GenerationPipelineResult } from './services/generationPipelineService';
import { parsePugStructure } from './services/generationPipelineService';

type ImprovementItem = {
  selector: string;
  improvement: string;
  screen: {
    pug: string;
    css: string;
    data: Record<string, unknown>;
  };
};

type StoryArgs = {
  improvements: ImprovementItem[];
};

const basePug = `div.dashboard-screen.d-flex.flex-column.bg-light
  header.d-flex.align-items-center.justify-content-between.p-3.bg-white.shadow-sm.mb-3
    div.d-flex.align-items-center
      img.logo(src="/api/project-images/assets/project-0689a10e121fbd43/img-f1e94c23c866dab5/v-09f265f8cba04376.png" alt="logo" style="height:48px;")
    div.user-info.d-flex.align-items-center.gap-3.bg-white.rounded.px-3.py-2.shadow-sm
      b-avatar(size="3rem" :src="user.avatar" rounded="circle")
      div
        strong Hola, {{ user.name }}!
      button.btn.btn-warning.circle-button(aria-label="Agregar crédito")
        i.bi.bi-credit-card-2-back
  section.matches-list.glass.rounded.shadow-sm.p-3
    div.match-card.d-flex.align-items-center.justify-content-between.p-2.mb-3.rounded.shadow-sm(v-for="match in filteredMatches" :key="match.id" :class="{'match-live': match.status==='live','match-closed': match.status==='closed','match-open': match.status==='open'}")
      div.d-flex.align-items-center.flex-grow-1.gap-2
        img.team-flag(src=match.home.flag alt=match.home.name style="width:24px; height:auto;")
        strong {{ match.home.name }}
        span.mx-1 vs
        img.team-flag(src=match.away.flag alt=match.away.name style="width:24px; height:auto;")
        strong {{ match.away.name }}
      div.odds.d-flex.gap-2.me-3
        button.btn.btn-outline-primary(v-for="odd in match.odds" :key="odd.type" :class="{'active-odd': odd.selected}" @click="match.status==='open' && popup('popup-apuesta')") {{ odd.type === 'home' ? match.home.name : odd.type === 'away' ? match.away.name : 'Empate' }} {{ odd.value.toFixed(2) }}`;

const baseCss = `.dashboard-screen {
  position: relative;
  min-height: 100vh;
}
.match-card { background-color: #f9fbff; border: 1px solid #dee2e6; }
.match-live { background-color: #d0ebff; }
.match-closed { background-color: #e9ecef; }
.match-open { background-color: #e6ffed; }
.odds .btn-outline-primary { background-color: #e7f1ff; border-color: #0d6efd; color: #0d6efd; }
.odds .active-odd { background-color: #0d6efd; color: #fff; }
.team-flag { display: inline-block; }`;

const baseData = {
  popup: () => undefined,
  filteredMatches: [
    {
      id: 1,
      home: { name: 'Argentina', flag: 'https://flagcdn.com/ar.png' },
      away: { name: 'Canada', flag: 'https://flagcdn.com/ca.png' },
      odds: [{ type: 'home', value: 2.1, selected: false }, { type: 'draw', value: 3.4, selected: false }, { type: 'away', value: 3.5, selected: false }],
      status: 'live',
    },
    {
      id: 2,
      home: { name: 'Francia', flag: 'https://flagcdn.com/fr.png' },
      away: { name: 'Alemania', flag: 'https://flagcdn.com/de.png' },
      odds: [{ type: 'home', value: 2.1, selected: false }, { type: 'draw', value: 3.1, selected: false }, { type: 'away', value: 3, selected: false }],
      status: 'closed',
    },
    {
      id: 3,
      home: { name: 'Brasil', flag: 'https://flagcdn.com/br.png' },
      away: { name: 'México', flag: 'https://flagcdn.com/mx.png' },
      odds: [{ type: 'home', value: 1.5, selected: true }, { type: 'draw', value: 3.42, selected: false }, { type: 'away', value: 3.5, selected: false }],
      status: 'open',
    },
  ],
  user: { avatar: 'https://randomuser.me/api/portraits/men/32.jpg', name: 'Alex' },
};

function toPipelineResult(pug: string, css: string, data: Record<string, unknown>): GenerationPipelineResult {
  const template = parsePugStructure(pug);
  return {
    template,
    imports: [],
    css,
    data,
    sourcePug: pug,
    messages: [],
    metadata: {
      usedTags: [],
      nonBootstrapTags: [],
      unresolvedTags: [],
    },
  };
}

const meta: Meta<StoryArgs> = {
  title: 'Generated Screen/UX Improvements Overlay',
  render: (args) => ({
    setup() {
      const loading = ref(true);
      const error = ref('');
      const appliedIndex = ref<number | null>(null);
      const hoverIndex = ref<number | null>(null);
      const base = ref<Component | null>(null);
      const previews = ref<Component[]>([]);
      const cleanups = ref<Array<GeneratedScreenView['installStyles']>>([]);

      const buildView = async (id: string, pug: string, css: string, data: Record<string, unknown>) => {
        const view = await buildGeneratedScreen(toPipelineResult(pug, css, data), { styleId: id });
        cleanups.value.push(view.installStyles);
        return markRaw(view.component);
      };

      onMounted(async () => {
        try {
          base.value = await buildView('sb-ux-base', basePug, baseCss, baseData);
          const next = await Promise.all(
            args.improvements.map((entry, index) =>
              buildView(`sb-ux-preview-${index}`, entry.screen.pug, entry.screen.css, entry.screen.data),
            ),
          );
          previews.value = next;
        } catch (err) {
          error.value = err instanceof Error ? err.message : 'No se pudo preparar la historia';
        } finally {
          loading.value = false;
        }
      });

      return () => {
        if (loading.value) {
          return h('div', 'Cargando previews...');
        }
        if (error.value) {
          return h('div', error.value);
        }

        const activeComponent =
          hoverIndex.value !== null
            ? previews.value[hoverIndex.value]
            : appliedIndex.value !== null
              ? previews.value[appliedIndex.value]
              : base.value;

        return h('div', { style: { padding: '1rem' } }, [
          h('div', { style: { marginBottom: '0.75rem', display: 'flex', gap: '0.5rem', flexWrap: 'wrap' } }, args.improvements.map((entry, i) =>
            h(
              'button',
              {
                type: 'button',
                onMouseenter: () => { hoverIndex.value = i; },
                onMouseleave: () => { hoverIndex.value = null; },
                onClick: () => { appliedIndex.value = i; hoverIndex.value = null; },
                style: {
                  borderRadius: '999px',
                  border: '1px solid #0ea5e9',
                  background: appliedIndex.value === i ? '#0ea5e9' : '#eff6ff',
                  color: appliedIndex.value === i ? '#fff' : '#0f172a',
                  padding: '0.35rem 0.7rem',
                  fontSize: '12px',
                  cursor: 'pointer',
                },
                title: entry.improvement,
              },
              `${i + 1}. ${entry.selector}`,
            ),
          )),
          activeComponent ? h(activeComponent) : null,
        ]);
      };
    },
  }),
  args: {
    improvements: [
      {
        selector: '.match-card .team-flag',
        improvement:
          'Add accessible tooltip (title/aria-label) showing the team name when hovering over each flag to improve discoverability and assistive‑technology support.',
        screen: {
          pug: basePug.replace(
            'img.team-flag(src=match.home.flag alt=match.home.name style="width:24px; height:auto;")',
            'img.team-flag(src=match.home.flag alt=match.home.name style="width:24px; height:auto;" title=match.home.name aria-label=match.home.name)',
          ).replace(
            'img.team-flag(src=match.away.flag alt=match.away.name style="width:24px; height:auto;")',
            'img.team-flag(src=match.away.flag alt=match.away.name style="width:24px; height:auto;" title=match.away.name aria-label=match.away.name)',
          ),
          css: baseCss,
          data: baseData,
        },
      },
      {
        selector: '.odds .btn-outline-primary.active-odd',
        improvement:
          'Animate the active odd button (e.g., subtle scale‑up or pulse) and enforce a contrast‑friendly color to make the selected bet more visually prominent.',
        screen: {
          pug: basePug,
          css: `${baseCss}
.odds .btn-outline-primary.active-odd {
  background-color: #0a58ca;
  border-color: #0a58ca;
  color: #ffffff;
  font-weight: 600;
  box-shadow: 0 0 0 3px rgba(13, 110, 253, 0.5);
  animation: oddPulse 0.6s ease-in-out infinite alternate;
  transition: transform 0.2s;
}
@keyframes oddPulse {
  from { transform: scale(1); }
  to { transform: scale(1.07); }
}`,
          data: baseData,
        },
      },
      {
        selector: '.match-card.match-open',
        improvement:
          'Add a hover effect (e.g., raise shadow and pointer cursor) on open matches to signal that the card is clickable and can open the betting popup.',
        screen: {
          pug: basePug,
          css: `${baseCss}
.match-card.match-open:hover {
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(0,0,0,0.2);
}`,
          data: baseData,
        },
      },
    ],
  },
};

export default meta;
type Story = StoryObj<StoryArgs>;

export const OverlayHoverAndApply: Story = {};
