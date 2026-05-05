import { h, markRaw, onMounted, onUnmounted, ref, type Component } from 'vue';
import type { Meta, StoryObj } from '@storybook/vue3';
import {
  buildGeneratedScreen,
  type GeneratedScreenView,
} from './services/generationRenderService';
import {
  parsePugStructure,
  type GenerationPipelineResult,
  type PugTemplateNode,
  type PugTemplateTree,
  type PipelineImport,
} from './services/generationPipelineService';
import VueChart from './components/charts/VueChart';

type StoryArgs = {
  pug: string;
  css: string;
  data: Record<string, unknown>;
  imports?: PipelineImport[];
};

function expect(condition: boolean, message: string): void {
  if (!condition) {
    throw new Error(message);
  }
}

function extractUsedTags(tree: PugTemplateTree): string[] {
  const usedTags = new Set<string>();
  const walk = (node: PugTemplateNode): void => {
    if (node.type === 'element') {
      usedTags.add(node.tag);
      for (const child of node.children) {
        walk(child);
      }
    }
  };

  for (const node of tree.children) {
    walk(node);
  }

  return Array.from(usedTags);
}

function buildPipelineResultFromArgs(args: StoryArgs): GenerationPipelineResult {
  const template = parsePugStructure(args.pug);
  return {
    template,
    imports: args.imports ?? [],
    css: args.css,
    data: args.data,
    sourcePug: args.pug,
    messages: [],
    metadata: {
      usedTags: extractUsedTags(template),
      nonBootstrapTags: [],
      unresolvedTags: [],
    },
  };
}

const meta: Meta<StoryArgs> = {
  title: 'Generated Screen/Desde Pug + CSS + Data',
  render: (args) => {
    return {
      setup() {
        const generated = ref<Component | null>(null);
        const status = ref('Renderizando componente generado...');
        const storyStyles = ref<GeneratedScreenView['installStyles'] | null>(null);

        onMounted(async () => {
          try {
            const view = await buildGeneratedScreen(buildPipelineResultFromArgs(args), {
              styleId: `storybook-generated-screen-${Date.now()}`,
              componentLoaders: {
                VueChart: () => Promise.resolve(VueChart),
              },
            });
            generated.value = markRaw(view.component);
            storyStyles.value = view.installStyles;
            status.value = 'Componente renderizado';
          } catch (error) {
            console.error('[storybook] Error building generated component', error);
            status.value = error instanceof Error ? error.message : 'No se pudo renderizar el componente';
          }
        });

        onUnmounted(() => {
          if (storyStyles.value) {
            storyStyles.value();
          }
        });

        return () =>
          generated.value
            ? h('div', { class: 'storybook-generated-screen' }, [h(generated.value)])
            : h('div', { class: 'storybook-generated-screen-loading' }, status.value);
      },
    };
  },
  args: {
    pug: [
      "div.story-root",
      "  h1 {{ heading }}",
      "  p.user {{ userName }}",
      "  p {{ description }}",
      "  button.btn.btn-primary(type='button') Botón desde el flujo generado",
      "  p.muted {{ user.city }}, {{ timestamp }}",
    ].join('\n'),
    css: [
      '.story-root {',
      '  padding: 1.2rem;',
      '  border-radius: 12px;',
      '  border: 1px dashed #6c8bd1;',
      '  background: #f6f8ff;',
      '  color: #1f2a44;',
      '}',
      '.story-root .user {',
      '  font-weight: 700;',
      '}',
      '.story-root .muted {',
      '  color: #54637d;',
      '}',
      '.storybook-generated-screen-loading {',
      '  padding: 1rem;',
      '  border: 1px dashed #8ca2ff;',
      '  color: #21356a;',
      '}',
    ].join('\n'),
    data: {
      heading: 'Vista de prueba en Storybook',
      userName: 'María Rodríguez',
      description: 'Esto renderiza a partir de un template Pug, CSS y un payload data.',
      user: {
        city: 'Madrid',
      },
      timestamp: new Date().toISOString(),
    },
  },
  argTypes: {
    pug: { control: 'text' },
    css: { control: 'text' },
    data: { control: 'object' },
  },
};

export const RenderDesdePugCssData: Story = {
  args: {
    ...meta.args,
  },
};

export const LoginFormWithPrependTemplates: Story = {
  args: {
    ...meta.args,
    pug: [
      'div.login-screen',
      '  h1.title {{ title }}',
      '  b-form(@submit.prevent="onLogin")',
      '    b-input-group.mb-3',
      '      template(#prepend)',
      '        span.input-group-text',
      '          i.bi.bi-envelope',
      '      b-form-input(type="email" v-model="form.email" placeholder="Email address" required)',
      '    b-input-group.mb-3',
      '      template(#prepend)',
      '        span.input-group-text',
      '          i.bi.bi-lock',
      '      b-form-input(type="password" v-model="form.password" placeholder="Password" required)',
      '    b-button.btn-primary.btn-block(type="submit") {{ buttonText }}',
      '  a.forgot-link(href="#") {{ forgotText }}',
    ].join('\n'),
    css: [
      '.login-screen {',
      '  max-width: 420px;',
      '  margin: 0 auto;',
      '  padding: 1.5rem;',
      '  border: 1px solid #d9e2f5;',
      '  border-radius: 12px;',
      '  background: #f8fbff;',
      '}',
      '.login-screen .title {',
      '  margin-bottom: 1rem;',
      '  font-size: 1.7rem;',
      '  font-weight: 700;',
      '}',
      '.login-screen .forgot-link {',
      '  display: inline-block;',
      '  margin-top: 0.75rem;',
      '  color: #2d5ec9;',
      '}',
      '.login-screen .btn-block {',
      '  width: 100%;',
      '}',
      '.login-screen .input-group-text {',
      '  border-right: 0;',
      '}',
      '.login-screen .bi {',
      '  color: #4b5563;',
      '}',
    ].join('\n'),
    data: {
      title: 'Acceso',
      buttonText: 'Entrar',
      forgotText: '¿Olvidaste tu contraseña?',
      form: {
        email: 'usuario@ejemplo.com',
        password: 'demo1234',
      },
      onLogin: () => {
        // no-op para story
      },
    },
  },
  play: async ({ canvasElement }) => {
    const root = canvasElement as HTMLElement;

    const hasEnvelopeIcon = !!root.querySelector('.input-group-text .bi.bi-envelope');
    if (!hasEnvelopeIcon) {
      throw new Error('No se renderizó el icono de email en el slot prepend.');
    }

    const hasLockIcon = !!root.querySelector('.input-group-text .bi.bi-lock');
    if (!hasLockIcon) {
      throw new Error('No se renderizó el icono de contraseña en el slot prepend.');
    }

    const form = root.querySelector('form');
    if (!form) {
      throw new Error('No se renderizó el formulario de login.');
    }
  },
};

export const BenchmarkScreen: Story = {
  args: {
    ...meta.args,
    imports: [
      {
        tag: 'VueChart',
        localName: 'VueChart',
        source: 'vue-chartjs',
        isCatalogResolved: true,
      },
    ],
    pug: [
      "div.benchmark-screen",
      "  b-card(class='my-3')",
      "    b-card-header",
      "      h4",
      "        i.bi.bi-speedometer2",
      "        |  Benchmark Results",
      "    b-card-body",
      "      b-button(variant='primary' @click='loadBenchmarks')",
      "        i.bi.bi-arrow-clockwise",
      "        |  Refresh",
      "      b-table(:items='benchmarks' :fields='fields' striped hover small)",
      "      div.chart-container",
      "        VueChart(chart-type='bar' :chart-data='chartData' :chart-options='chartOptions')",
    ].join('\n'),
    css: [
      '.benchmark-screen {',
      '  padding: 1rem;',
      '}',
      '.benchmark-screen .my-3 {',
      '  box-shadow: 0 12px 24px rgba(17, 24, 39, 0.08);',
      '}',
      '.benchmark-screen .chart-container {',
      '  margin-top: 1rem;',
      '  min-height: 300px;',
      '}',
      '.benchmark-screen .bi {',
      '  margin-right: 0.4rem;',
      '}',
    ].join('\n'),
    data: {
      benchmarks: [
        { bench: 'Login render', avgMs: 126, status: 'ok' },
        { bench: 'Dashboard', avgMs: 248, status: 'ok' },
        { bench: 'Export', avgMs: 412, status: 'warn' },
      ],
      fields: [
        { key: 'bench', label: 'Benchmark' },
        { key: 'avgMs', label: 'Avg (ms)' },
        { key: 'status', label: 'Status' },
      ],
      loadBenchmarks: () => {
        // no-op para story
      },
      chartData: {
        labels: ['Login', 'Dashboard', 'Export'],
        datasets: [
          {
            label: 'Latencia (ms)',
            data: [126, 248, 412],
            backgroundColor: ['#3b82f6', '#f59e0b', '#ef4444'],
          },
        ],
      },
      chartOptions: {
        responsive: true,
        maintainAspectRatio: false,
        scales: {
          y: {
            beginAtZero: true,
          },
        },
      },
    },
  },
  play: async ({ canvasElement }) => {
    const root = canvasElement as HTMLElement;
    const table = root.querySelector('table');
    if (!table) {
      throw new Error('No se renderizó la tabla de benchmarks.');
    }

    const canvas = root.querySelector('canvas');
    if (!canvas) {
      throw new Error('No se renderizó el canvas del gráfico.');
    }
  },
};

export const RenderTableWithActions: Story = {
  args: {
    ...meta.args,
    pug: [
      "div.container",
      "  b-table(:items='courses' :fields='fields' small responsive)",
      "    template(v-slot:cell(acciones)='{ item }')",
      "      b-button(@click='onEquivalences(item)') Equivalencias",
      "  b-button.mt-2(@click='reprocess') Reprocess",
    ].join('\n'),
    data: {
      courses: [
        { codigo: 'MAT101', nombre: 'Cálculo I', creditos: 4 },
        { codigo: 'FIS202', nombre: 'Física General', creditos: 3 },
      ],
      fields: [
        { key: 'codigo', label: 'Código' },
        { key: 'nombre', label: 'Curso' },
        { key: 'creditos', label: 'Créditos' },
        { key: 'acciones', label: 'Acciones' },
      ],
      onEquivalences: (item: Record<string, unknown>) => {
        console.log('onEquivalences', item);
      },
      reprocess: () => {
        console.log('reprocess');
      },
    },
    css: [
      '.container {',
      '  padding: 1rem;',
      '}',
      '.container .mt-2 {',
      '  margin-top: 0.75rem;',
      '}',
    ].join('\n'),
  },
  play: async ({ canvasElement }) => {
    const root = canvasElement as HTMLElement;
    const findButtons = (label: string) =>
      Array.from(root.querySelectorAll('button')).filter((button) =>
        button.textContent?.trim().toLowerCase() === label.toLowerCase(),
      );

    const reprocessButtons = findButtons('Reprocess');
    if (reprocessButtons.length === 0) {
      throw new Error('No se encontró el botón Reprocess.');
    }
    reprocessButtons[0].click();

    const actionButtons = findButtons('Equivalencias');
    if (actionButtons.length === 0) {
      throw new Error('No se encontró el botón Equivalencias.');
    }
    actionButtons[0].click();
  },
};

export const CoursesTableWithModal: Story = {
  args: {
    ...meta.args,
    pug: [
      'b-table(:items="courses" :fields="fields" striped hover small)',
      '  template(#cell(actions)="data")',
      '    b-button(@click="showDetails(data.item)" variant="primary" size="sm") Details',
      '    b-button(@click="deleteCourse(data.item)" variant="danger" size="sm" class="ml-2") Delete',
      'b-modal(id="course-modal" v-model="showModal" title="Course Details")',
      '  p Code: {{ selectedCourse.code }}',
      '  p Name: {{ selectedCourse.name }}',
      '  p Credits: {{ selectedCourse.credits }}',
    ].join('\n'),
    css: [
      '.b-table {',
      '  width: 100%;',
      '}',
      '.b-button {',
      '  margin: 0;',
      '}',
    ].join('\n'),
    data: {
      courses: [
        {
          code: 'CS101',
          credits: 3,
          name: 'Intro to Computer Science',
        },
        {
          code: 'MATH201',
          credits: 4,
          name: 'Calculus II',
        },
        {
          code: 'ENG150',
          credits: 2,
          name: 'English Literature',
        },
      ],
      fields: [
        {
          key: 'code',
          label: 'Code',
          sortable: true,
        },
        {
          key: 'name',
          label: 'Name',
          sortable: true,
        },
        {
          key: 'credits',
          label: 'Credits',
          sortable: true,
        },
        {
          key: 'actions',
          label: 'Actions',
        },
      ],
      selectedCourse: {
        code: '',
        credits: null,
        name: '',
      },
      showModal: false,
      showDetails: (item: Record<string, unknown>) => {
        console.log('showDetails', item);
      },
      deleteCourse: (item: Record<string, unknown>) => {
        console.log('deleteCourse', item);
      },
    },
  },
  play: async ({ canvasElement }) => {
    const root = canvasElement as HTMLElement;
    const getTextNodes = () => root.textContent?.toLowerCase() ?? '';
    const content = getTextNodes();

    const hasCourses = content.includes('cs101') && content.includes('math201') && content.includes('eng150');
    expect(hasCourses, 'No se están mostrando los cursos esperados en la tabla.');

    const buttons = Array.from(root.querySelectorAll('button')).map((button) => button.textContent?.trim() ?? '');
    const hasDetails = buttons.includes('Details');
    const hasDelete = buttons.includes('Delete');

    expect(hasDetails && hasDelete, 'No se detectaron los botones Details/Delete en la tabla.');
  },
};

type Story = StoryObj<StoryArgs>;
export default meta;
