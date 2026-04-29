import { setup } from '@storybook/vue3';
import { createBootstrap } from 'bootstrap-vue-next';
import type { Preview } from '@storybook/vue3';

import 'bootstrap/dist/css/bootstrap.min.css';
import 'bootstrap-vue-next/dist/bootstrap-vue-next.css';

setup((app) => {
  app.use(createBootstrap());
});

const preview: Preview = {};

export default preview;

