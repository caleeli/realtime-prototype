package registry

const CatalogVersion = "2026-04-25"

var initialComponentInventory = []ComponentInventoryItem{
	{
		Name:   "DateRangePicker",
		Module: "advanced-inputs",
		Tag:    "DateRangePicker",
		Pack:   "advanced-inputs",
		Props: []ComponentPropMetadata{
			{Name: "modelValue", Type: "string | [Date, Date] | null", Required: false, Description: strPtr("Current selected date range.")},
			{Name: "placeholder", Type: "string", Required: false, Default: strPtr("Start date - End date"), Description: strPtr("Input placeholder text.")},
			{Name: "clearable", Type: "boolean", Required: false, Default: strPtr("true"), Description: strPtr("Show clear button.")},
			{Name: "format", Type: "string", Required: false, Default: strPtr("YYYY-MM-DD"), Description: strPtr("Date format passed to formatter.")},
		},
		Slots: []ComponentSlotMetadata{
			{Name: "header", Description: strPtr("Replace the calendar header area." )},
			{Name: "footer", Description: strPtr("Append custom action buttons.")},
		},
		Events: []ComponentEventMetadata{
			{Name: "update:modelValue", Payload: strPtr("{ start?: string; end?: string }"), Description: strPtr("Emits when range changes.")},
			{Name: "apply", Payload: strPtr("{ start: string; end: string }"), Description: strPtr("Emits when user applies a range.")},
		},
		Examples: []ComponentExample{
			{Label: "Report period chooser", Pug: "DateRangePicker(v-model='reportRange' :clearable='true' format='YYYY-MM-DD')", Description: strPtr("Basic form usage in form groups.")},
		},
		Restrictions: []ComponentRestriction{
			{Type: RestrictionRuntime, Message: "Should not be used inside SSR-only rendering paths."},
			{Type: RestrictionUX, Message: "Pair with clear reset controls when used in filter forms."},
		},
		Enabled: true,
		Version: strPtr("1.2.0"),
	},
	{
		Name:   "AsyncMultiSelect",
		Module: "advanced-inputs",
		Tag:    "AsyncMultiSelect",
		Pack:   "advanced-inputs",
		Props: []ComponentPropMetadata{
			{Name: "modelValue", Type: "string[]", Required: false, Description: strPtr("Selected IDs.")},
			{Name: "search", Type: "(query: string) => Promise<Option[]>", Required: true, Description: strPtr("Async remote query function.")},
			{Name: "debounceMs", Type: "number", Required: false, Default: strPtr("300"), Description: strPtr("Delay between remote requests.")},
			{Name: "labelKey", Type: "string", Required: false, Default: strPtr("label"), Description: strPtr("Option label field.")},
			{Name: "valueKey", Type: "string", Required: false, Default: strPtr("value"), Description: strPtr("Option value field.")},
		},
		Slots: []ComponentSlotMetadata{
			{Name: "option", Description: strPtr("Custom rendering for each suggestion item.")},
			{Name: "selected", Description: strPtr("Custom rendering for selected tokens.")},
		},
		Events: []ComponentEventMetadata{
			{Name: "search", Payload: strPtr("string"), Description: strPtr("Fires whenever query changes.")},
			{Name: "update:modelValue", Payload: strPtr("string[]"), Description: strPtr("Selection changes.")},
		},
		Examples: []ComponentExample{
			{Label: "Team member picker", Pug: "AsyncMultiSelect(v-model='selectedUsers' :search='fetchUsers' label-key='fullName' value-key='id')", Description: strPtr("Load options as user types.")},
		},
		Restrictions: []ComponentRestriction{
			{Type: RestrictionRuntime, Message: "Requires a function prop that resolves within 15s."},
			{Type: RestrictionSecurity, Message: "Search input is sanitized through backend API proxy."},
		},
		Enabled: true,
		Version: strPtr("1.2.0"),
	},
	{
		Name:   "InputMask",
		Module: "advanced-inputs",
		Tag:    "InputMask",
		Pack:   "advanced-inputs",
		Props: []ComponentPropMetadata{
			{Name: "modelValue", Type: "string", Required: false, Description: strPtr("Masked string value.")},
			{Name: "mask", Type: "string", Required: true, Description: strPtr("Mask pattern, e.g. (999) 999-9999")},
			{Name: "placeholder", Type: "string", Required: false, Default: strPtr("Enter value"), Description: strPtr("Placeholder while empty.")},
			{Name: "unmask", Type: "boolean", Required: false, Default: strPtr("false"), Description: strPtr("Return unmasked value on input.")},
		},
		Slots: []ComponentSlotMetadata{
			{Name: "addon", Description: strPtr("Custom input addon slot.")},
		},
		Events: []ComponentEventMetadata{
			{Name: "complete", Payload: strPtr("string"), Description: strPtr("All mask characters completed.")},
			{Name: "update:modelValue", Payload: strPtr("string"), Description: strPtr("Output value updates.")},
		},
		Examples: []ComponentExample{
			{Label: "Phone field", Pug: "InputMask(v-model='phone' mask='(999) 999-9999' placeholder='(201) 555-0123')", Description: strPtr("Use for strict format fields.")},
		},
		Restrictions: []ComponentRestriction{
			{Type: RestrictionUX, Message: "Use with short fields and helper text for expected format."},
			{Type: RestrictionSecurity, Message: "Do not bind secrets or tokens to this component."},
		},
		Enabled: true,
		Version: strPtr("1.2.0"),
	},
	{
		Name:   "pm-table",
		Module: "packages/component-registry",
		Tag:    "pm-table",
		Pack:   "files",
		Props: []ComponentPropMetadata{
			{Name: "modelValue", Type: "Array<Record<string, any>>", Required: false, Description: strPtr("Rows being displayed.")},
			{Name: "totalRows", Type: "number", Required: false, Description: strPtr("Total matching rows from server.")},
			{Name: "sortBy", Type: "string[]", Required: false, Description: strPtr("Sort columns order.")},
			{Name: "sortDesc", Type: "boolean[]", Required: false, Description: strPtr("Sort directions.")},
			{Name: "currentPage", Type: "number", Required: false, Default: strPtr("1"), Description: strPtr("Current page index.")},
			{Name: "perPage", Type: "number", Required: false, Default: strPtr("10"), Description: strPtr("Page size.")},
			{Name: "filter", Type: "Record<string, any>", Required: false, Description: strPtr("Server-side filter payload.")},
			{Name: "busy", Type: "boolean", Required: false, Default: strPtr("false"), Description: strPtr("Loading state.")},
		},
		Slots: []ComponentSlotMetadata{
			{Name: "cell", Description: strPtr("Custom cell rendering.")},
			{Name: "empty", Description: strPtr("Shown when dataset is empty.")},
		},
		Events: []ComponentEventMetadata{
			{Name: "sort-change", Payload: strPtr("{ sortBy: string[], sortDesc: boolean[] }"), Description: strPtr("User changed sorting criteria.")},
			{Name: "filter-change", Payload: strPtr("Record<string, any>"), Description: strPtr("User changed filter values.")},
			{Name: "page-change", Payload: strPtr("number"), Description: strPtr("Page number changed.")},
			{Name: "update:items", Payload: strPtr("Array<Record<string, any>>"), Description: strPtr("Updated data slice received from server.")},
		},
		Examples: []ComponentExample{
			{Label: "Server paginated list", Pug: "pm-table(:current-page='page' :per-page='10' :busy='loading' @sort-change='onSort' @page-change='onPage' @filter-change='onFilter')", Description: strPtr("Handle all server interactions on change handlers.")},
		},
		Restrictions: []ComponentRestriction{
			{Type: RestrictionRuntime, Message: "Must use with backend paging/sorting API endpoint that honors query params."},
			{Type: RestrictionRuntime, Message: "Server must return `items` and `totalRows` consistently."},
			{Type: RestrictionSecurity, Message: "Validate all query params before forwarding to backend data source."},
		},
		Enabled: true,
		Version: strPtr("0.4.2"),
	},
	{
		Name:   "DropzoneUploader",
		Module: "vue-dropzone",
		Tag:    "DropzoneUploader",
		Pack:   "files",
		Props: []ComponentPropMetadata{
			{Name: "uploadUrl", Type: "string", Required: true, Description: strPtr("Target upload endpoint.")},
			{Name: "maxFileSize", Type: "number", Required: false, Default: strPtr("10485760"), Description: strPtr("Max bytes per file.")},
			{Name: "acceptedFiles", Type: "string", Required: false, Default: strPtr("*/*"), Description: strPtr("Accepted MIME types.")},
			{Name: "multiple", Type: "boolean", Required: false, Default: strPtr("true"), Description: strPtr("Allow multiple uploads.")},
		},
		Slots: []ComponentSlotMetadata{
			{Name: "thumb", Description: strPtr("Custom preview template.")},
			{Name: "status", Description: strPtr("Upload status and error messages.")},
		},
		Events: []ComponentEventMetadata{
			{Name: "upload-success", Payload: strPtr("{ id: string, file: object }"), Description: strPtr("File uploaded successfully.")},
			{Name: "upload-error", Payload: strPtr("{ id: string, error: string }"), Description: strPtr("Upload failed.")},
			{Name: "update:files", Payload: strPtr("Array<object>"), Description: strPtr("Selected/uploaded list changed.")},
		},
		Examples: []ComponentExample{
			{Label: "Avatar uploader", Pug: "DropzoneUploader(upload-url='/api/files' :max-file-size='5242880' accepted-files='image/*' :multiple='false')", Description: strPtr("Single image upload with MIME filtering.")},
		},
		Restrictions: []ComponentRestriction{
			{Type: RestrictionRuntime, Message: "Uploads must be validated server-side before persistence."},
			{Type: RestrictionSecurity, Message: "Only allow known-safe MIME types and sanitize file names."},
		},
		Enabled: true,
		Version: strPtr("0.9.8"),
	},
	{
		Name:   "VueChart",
		Module: "vue-chartjs",
		Tag:    "VueChart",
		Pack:   "charts",
		Props: []ComponentPropMetadata{
			{Name: "chartType", Type: "'line' | 'bar' | 'pie' | 'doughnut' | 'radar' | 'polarArea' | 'bubble' | 'scatter'", Required: false, Default: strPtr("line"), Description: strPtr("Chart.js chart type to render.")},
			{Name: "type", Type: "string", Required: false, Description: strPtr("Alias for chartType.")},
			{Name: "chartData", Type: "Record<string, any> | null", Required: true, Description: strPtr("Data object for vue-chartjs/chart.js")},
			{Name: "chartOptions", Type: "Record<string, any>", Required: false, Description: strPtr("Options object for chart.js.")},
			{Name: "options", Type: "Record<string, any>", Required: false, Description: strPtr("Alias for chartOptions.")},
			{Name: "width", Type: "number | string", Required: false, Description: strPtr("Optional rendered chart width.")},
			{Name: "height", Type: "number | string", Required: false, Description: strPtr("Optional rendered chart height.")},
		},
		Slots:  []ComponentSlotMetadata{},
		Events: []ComponentEventMetadata{},
		Examples: []ComponentExample{
			{
				Label:       "Sales chart",
				Pug:         "VueChart(chart-type='line' :chart-data='salesData' :chart-options='salesOptions')",
				Description: strPtr("Render a line chart bound to data variables."),
			},
		},
		Restrictions: []ComponentRestriction{
			{Type: RestrictionRuntime, Message: "chart-data must be compatible with vue-chartjs datasets format for the selected chart-type."},
			{Type: RestrictionSecurity, Message: "Avoid untrusted config objects in chartOptions and keep values JSON serializable."},
		},
		Enabled: true,
		Version: strPtr("1.0.0"),
	},
}

func strPtr(value string) *string {
	v := value
	return &v
}

