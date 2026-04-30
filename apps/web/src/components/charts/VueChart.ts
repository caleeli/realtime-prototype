import { computed, defineComponent, h, type PropType } from 'vue';
import { Line, Bar, Pie, Doughnut, Radar, PolarArea, Bubble, Scatter } from 'vue-chartjs';
import 'chart.js/auto';

type SupportedChartType = 'line' | 'bar' | 'pie' | 'doughnut' | 'radar' | 'polararea' | 'bubble' | 'scatter';

interface ChartJsProps {
  chartType?: SupportedChartType | string;
  type?: string;
  chartData?: Record<string, unknown> | null;
  chartOptions?: Record<string, unknown>;
  options?: Record<string, unknown>;
  width?: number | string;
  height?: number | string;
}

export default defineComponent({
  name: 'VueChart',
  props: {
    chartType: String,
    type: String,
    chartData: {
      type: Object as unknown as PropType<Record<string, unknown> | null>,
      default: () => null,
    },
    chartOptions: {
      type: Object as unknown as PropType<Record<string, unknown>>,
      default: () => undefined,
    },
    options: {
      type: Object as unknown as PropType<Record<string, unknown>>,
      default: () => undefined,
    },
    width: {
      type: [String, Number] as unknown as PropType<number | string>,
      required: false,
    },
    height: {
      type: [String, Number] as unknown as PropType<number | string>,
      required: false,
    },
  },
  setup(props: Readonly<ChartJsProps>) {
    const dataPayload = computed<Record<string, unknown>>(() => {
      const value = props.chartData;
      return value === null || value === undefined
        ? {
            labels: [],
            datasets: [],
          }
        : value;
    });

    const optionsPayload = computed<Record<string, unknown>>(() => {
      return props.chartOptions ?? props.options ?? {};
    });

    const chartType = computed<SupportedChartType>(() => {
      const source = (props.chartType ?? props.type ?? 'line').toLowerCase();
      if (source === 'polararea' || source === 'polar_area' || source === 'polar-area') {
        return 'polararea';
      }
      return source as SupportedChartType;
    });

    const Renderer = computed(() => {
      if (chartType.value === 'line') return Line;
      if (chartType.value === 'bar') return Bar;
      if (chartType.value === 'pie') return Pie;
      if (chartType.value === 'doughnut') return Doughnut;
      if (chartType.value === 'radar') return Radar;
      if (chartType.value === 'polararea') return PolarArea;
      if (chartType.value === 'bubble') return Bubble;
      return Scatter;
    });

    return () =>
      dataPayload.value.datasets && (dataPayload.value.datasets as Array<unknown>).length === 0
        ? h('div', { class: 'vue-chart-empty', 'aria-live': 'polite' }, 'No hay datos para el gráfico')
        : h(Renderer.value, {
            data: dataPayload.value,
            options: optionsPayload.value,
            width: props.width,
            height: props.height,
          });
  },
});
