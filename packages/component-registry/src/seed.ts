import type { ComponentInventoryItem } from './types';

export const CATALOG_VERSION = '2026-04-25';

export const initialComponentInventory: ComponentInventoryItem[] = [
  {
    name: 'DateRangePicker',
    module: 'advanced-inputs',
    tag: 'DateRangePicker',
    pack: 'advanced-inputs',
    props: [
      { name: 'modelValue', type: 'string | [Date, Date] | null', required: false, description: 'Current selected date range.' },
      { name: 'placeholder', type: 'string', required: false, defaultValue: 'Start date - End date', description: 'Input placeholder text.' },
      { name: 'clearable', type: 'boolean', required: false, defaultValue: 'true', description: 'Show clear button.' },
      { name: 'format', type: 'string', required: false, defaultValue: 'YYYY-MM-DD', description: 'Date format passed to formatter.' },
    ],
    slots: [
      { name: 'header', description: 'Replace the calendar header area.' },
      { name: 'footer', description: 'Append custom action buttons.' },
    ],
    events: [
      { name: 'update:modelValue', payload: '{ start?: string; end?: string }', description: 'Emits when range changes.' },
      { name: 'apply', payload: '{ start: string; end: string }', description: 'Emits when user applies a range.' },
    ],
    examples: [
      {
        label: 'Report period chooser',
        pug: "DateRangePicker(v-model='reportRange' :clearable='true' format='YYYY-MM-DD')",
        description: 'Basic form usage in form groups.',
      },
    ],
    restrictions: [
      { type: 'runtime', message: 'Should not be used inside SSR-only rendering paths.' },
      { type: 'ux', message: 'Pair with clear reset controls when used in filter forms.' },
    ],
    enabled: true,
    version: '1.2.0',
  },
  {
    name: 'AsyncMultiSelect',
    module: 'advanced-inputs',
    tag: 'AsyncMultiSelect',
    pack: 'advanced-inputs',
    props: [
      { name: 'modelValue', type: 'string[]', required: false, description: 'Selected IDs.' },
      { name: 'search', type: '(query: string) => Promise<Option[]>', required: true, description: 'Async remote query function.' },
      { name: 'debounceMs', type: 'number', required: false, defaultValue: '300', description: 'Delay between remote requests.' },
      { name: 'labelKey', type: 'string', required: false, defaultValue: 'label', description: 'Option label field.' },
      { name: 'valueKey', type: 'string', required: false, defaultValue: 'value', description: 'Option value field.' },
    ],
    slots: [
      { name: 'option', description: 'Custom rendering for each suggestion item.' },
      { name: 'selected', description: 'Custom rendering for selected tokens.' },
    ],
    events: [
      { name: 'search', payload: 'string', description: 'Fires whenever query changes.' },
      { name: 'update:modelValue', payload: 'string[]', description: 'Selection changes.' },
    ],
    examples: [
      {
        label: 'Team member picker',
        pug: "AsyncMultiSelect(v-model='selectedUsers' :search='fetchUsers' label-key='fullName' value-key='id')",
        description: 'Load options as user types.',
      },
    ],
    restrictions: [
      { type: 'runtime', message: 'Requires a function prop that resolves within 15s.' },
      { type: 'security', message: 'Search input is sanitized through backend API proxy.' },
    ],
    enabled: true,
    version: '1.2.0',
  },
  {
    name: 'InputMask',
    module: 'advanced-inputs',
    tag: 'InputMask',
    pack: 'advanced-inputs',
    props: [
      { name: 'modelValue', type: 'string', required: false, description: 'Masked string value.' },
      { name: 'mask', type: 'string', required: true, description: 'Mask pattern, e.g. (999) 999-9999' },
      { name: 'placeholder', type: 'string', required: false, defaultValue: 'Enter value', description: 'Placeholder while empty.' },
      { name: 'unmask', type: 'boolean', required: false, defaultValue: 'false', description: 'Return unmasked value on input.' },
    ],
    slots: [{ name: 'addon', description: 'Custom input addon slot.' }],
    events: [
      { name: 'complete', payload: 'string', description: 'All mask characters completed.' },
      { name: 'update:modelValue', payload: 'string', description: 'Output value updates.' },
    ],
    examples: [
      {
        label: 'Phone field',
        pug: "InputMask(v-model='phone' mask='(999) 999-9999' placeholder='(201) 555-0123')",
        description: 'Use for strict format fields.' ,
      },
    ],
    restrictions: [
      { type: 'ux', message: 'Use with short fields and helper text for expected format.' },
      { type: 'security', message: 'Do not bind secrets or tokens to this component.' },
    ],
    enabled: true,
    version: '1.2.0',
  },
  {
    name: 'pm-table',
    module: 'packages/component-registry',
    tag: 'pm-table',
    pack: 'files',
    props: [
      { name: 'modelValue', type: 'Array<Record<string, any>>', required: false, description: 'Rows being displayed.' },
      { name: 'totalRows', type: 'number', required: false, description: 'Total matching rows from server.' },
      { name: 'sortBy', type: 'string[]', required: false, description: 'Sort columns order.' },
      { name: 'sortDesc', type: 'boolean[]', required: false, description: 'Sort directions.' },
      { name: 'currentPage', type: 'number', required: false, defaultValue: '1', description: 'Current page index.' },
      { name: 'perPage', type: 'number', required: false, defaultValue: '10', description: 'Page size.' },
      { name: 'filter', type: 'Record<string, any>', required: false, description: 'Server-side filter payload.' },
      { name: 'busy', type: 'boolean', required: false, defaultValue: 'false', description: 'Loading state.' },
    ],
    slots: [
      { name: 'cell', description: 'Custom cell rendering.' },
      { name: 'empty', description: 'Shown when dataset is empty.' },
    ],
    events: [
      { name: 'sort-change', payload: '{ sortBy: string[], sortDesc: boolean[] }', description: 'User changed sorting criteria.' },
      { name: 'filter-change', payload: 'Record<string, any>', description: 'User changed filter values.' },
      { name: 'page-change', payload: 'number', description: 'Page number changed.' },
      { name: 'update:items', payload: 'Array<Record<string, any>>', description: 'Updated data slice received from server.' },
    ],
    examples: [
      {
        label: 'Server paginated list',
        pug: "pm-table(:current-page='page' :per-page='10' :busy='loading' @sort-change='onSort' @page-change='onPage' @filter-change='onFilter')",
        description: 'Handle all server interactions on change handlers.',
      },
    ],
    restrictions: [
      { type: 'runtime', message: 'Must use with backend paging/sorting API endpoint that honors query params.' },
      { type: 'runtime', message: 'Server must return `items` and `totalRows` consistently.' },
      { type: 'security', message: 'Validate all query params before forwarding to backend data source.' },
    ],
    enabled: true,
    version: '0.4.2',
  },
  {
    name: 'DropzoneUploader',
    module: 'vue-dropzone',
    tag: 'DropzoneUploader',
    pack: 'files',
    props: [
      { name: 'uploadUrl', type: 'string', required: true, description: 'Target upload endpoint.' },
      { name: 'maxFileSize', type: 'number', required: false, defaultValue: '10485760', description: 'Max bytes per file.' },
      { name: 'acceptedFiles', type: 'string', required: false, defaultValue: '*/*', description: 'Accepted MIME types.' },
      { name: 'multiple', type: 'boolean', required: false, defaultValue: 'true', description: 'Allow multiple uploads.' },
    ],
    slots: [
      { name: 'thumb', description: 'Custom preview template.' },
      { name: 'status', description: 'Upload status and error messages.' },
    ],
    events: [
      { name: 'upload-success', payload: '{ id: string, file: object }', description: 'File uploaded successfully.' },
      { name: 'upload-error', payload: '{ id: string, error: string }', description: 'Upload failed.' },
      { name: 'update:files', payload: 'Array<object>', description: 'Selected/uploaded list changed.' },
    ],
    examples: [
      {
        label: 'Avatar uploader',
        pug: "DropzoneUploader(upload-url='/api/files' :max-file-size='5242880' accepted-files='image/*' :multiple='false')",
        description: 'Single image upload with MIME filtering.',
      },
    ],
    restrictions: [
      { type: 'runtime', message: 'Uploads must be validated server-side before persistence.' },
      { type: 'security', message: 'Only allow known-safe MIME types and sanitize file names.' },
    ],
    enabled: true,
    version: '0.9.8',
  },
];
