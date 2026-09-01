<template>
  <section class="schema-field">
    <template v-if="kind === 'object'">
      <h4 v-if="label">{{ label }} <span v-if="required" class="required">*</span></h4>
      <p v-if="fieldHelp" class="description">{{ fieldHelp }}</p>
      <div v-if="propertyEntries.length" class="object-fields">
        <SchemaField
          v-for="entry in propertyEntries"
          :key="entry.name"
          :schema="entry.schema"
          :root-schema="rootSchema"
          :model-value="objectValue[entry.name]"
          :label="entry.schema.title || entry.name"
          :required="requiredProperties.includes(entry.name)"
          :ui-schema="entry.uiSchema"
          :path="entry.path"
          :help-overrides="helpOverrides"
          @update:model-value="updateProperty(entry.name, $event)"
        />
      </div>
      <el-input
        v-else
        type="textarea"
        :rows="4"
        :model-value="jsonValue"
        @change="updateJSON"
      />
    </template>

    <template v-else-if="kind === 'array'">
      <h4>{{ label }} <span v-if="required" class="required">*</span></h4>
      <p v-if="fieldHelp" class="description">{{ fieldHelp }}</p>
      <el-card v-for="(item, index) in arrayValue" :key="index" class="array-item" shadow="never">
        <SchemaField
          :schema="effectiveSchema.items || {}"
          :root-schema="rootSchema"
          :model-value="item"
          :label="`${label || 'item'} ${index + 1}`"
          :ui-schema="effectiveUISchema.items || {}"
          :path="`${path}/${index}`"
          :help-overrides="helpOverrides"
          @update:model-value="updateArrayItem(index, $event)"
        />
        <el-button type="danger" plain size="small" @click="removeArrayItem(index)">{{ T('Remove') }}</el-button>
      </el-card>
      <el-button plain size="small" @click="addArrayItem">{{ T('AddItem', { item: label || T('Item') }) }}</el-button>
    </template>

    <el-form-item v-else :label="label" :required="required">
      <el-radio-group
        v-if="effectiveSchema.enum && widget === 'radio'"
        :model-value="modelValue"
        :disabled="fieldDisabled"
        @update:model-value="updateScalar"
      >
        <el-radio v-for="option in effectiveSchema.enum" :key="String(option)" :value="option">{{ String(option) }}</el-radio>
      </el-radio-group>
      <el-select
        v-else-if="effectiveSchema.enum"
        :model-value="modelValue"
        clearable
        :disabled="fieldDisabled"
        :placeholder="effectiveUISchema['ui:placeholder']"
        @update:model-value="updateScalar"
      >
        <el-option v-for="option in effectiveSchema.enum" :key="String(option)" :label="String(option)" :value="option" />
      </el-select>
      <el-radio-group
        v-else-if="kind === 'boolean' && widget === 'radio'"
        :model-value="modelValue"
        :disabled="fieldDisabled"
        @update:model-value="updateScalar"
      >
        <el-radio :value="true">{{ T('Enable') }}</el-radio>
        <el-radio :value="false">{{ T('Disable') }}</el-radio>
      </el-radio-group>
      <el-switch
        v-else-if="kind === 'boolean'"
        :model-value="Boolean(modelValue)"
        :disabled="fieldDisabled"
        @update:model-value="updateScalar"
      />
      <el-input-number
        v-else-if="kind === 'integer' || kind === 'number'"
        :model-value="modelValue"
        :min="effectiveSchema.minimum"
        :max="effectiveSchema.maximum"
        :step="effectiveSchema.multipleOf"
        :disabled="fieldDisabled"
        @update:model-value="updateScalar"
      />
      <el-input
        v-else
        :type="widget === 'password' ? 'password' : widget === 'textarea' ? 'textarea' : 'text'"
        :model-value="modelValue == null ? '' : String(modelValue)"
        :disabled="fieldDisabled"
        :placeholder="effectiveUISchema['ui:placeholder']"
        :show-password="widget === 'password'"
        @update:model-value="updateScalar"
      />
      <div v-if="fieldHelp" class="description">{{ fieldHelp }}</div>
    </el-form-item>
  </section>
</template>

<script setup>
import { computed } from 'vue'
import { T } from '@/utils/i18n'

const props = defineProps({
  schema: { type: Object, default: () => ({}) },
  rootSchema: { type: Object, required: true },
  modelValue: { default: undefined },
  label: { type: String, default: '' },
  required: { type: Boolean, default: false },
  uiSchema: { type: Object, default: () => ({}) },
  path: { type: String, default: '' },
  helpOverrides: { type: Object, default: () => ({}) },
})

const emit = defineEmits(['update:modelValue'])

function resolvePointer (reference) {
  if (!reference?.startsWith('#/')) return {}
  return reference.slice(2).split('/').reduce((value, part) => {
    const key = part.replace(/~1/g, '/').replace(/~0/g, '~')
    return value?.[key]
  }, props.rootSchema) || {}
}

function matchesCondition (condition, value) {
  if (!condition) return true
  if (condition.required && !condition.required.every(key => value && Object.prototype.hasOwnProperty.call(value, key))) {
    return false
  }
  return Object.entries(condition.properties || {}).every(([key, rule]) => {
    const candidate = value?.[key]
    if (rule.const !== undefined) return candidate === rule.const
    if (rule.enum) return rule.enum.includes(candidate)
    if (rule.type && candidate !== undefined) return valueMatchesType(candidate, rule.type)
    return true
  })
}

function valueMatchesType (value, type) {
  const types = Array.isArray(type) ? type : [type]
  return types.some(candidate => {
    if (candidate === 'null') return value == null
    if (candidate === 'array') return Array.isArray(value)
    if (candidate === 'object') return value !== null && typeof value === 'object' && !Array.isArray(value)
    if (candidate === 'integer') return Number.isInteger(value)
    if (candidate === 'number') return typeof value === 'number'
    return typeof value === candidate
  })
}

function variantScore (schema, value) {
  const resolved = materialize(schema, value)
  let score = 0
  if (resolved.type && valueMatchesType(value, resolved.type)) score += 4
  if (resolved.const !== undefined && value === resolved.const) score += 8
  if (resolved.enum?.includes(value)) score += 6
  if (resolved.required?.every(key => value && Object.prototype.hasOwnProperty.call(value, key))) score += resolved.required.length * 2
  if (matchesCondition(resolved, value)) score += 1
  return score
}

function mergeSchema (base, addition) {
  return {
    ...base,
    ...addition,
    properties: { ...(base.properties || {}), ...(addition.properties || {}) },
    required: [...new Set([...(base.required || []), ...(addition.required || [])])],
  }
}

function materialize (input, value, seen = new Set()) {
	if (input === true) return {}
	if (input === false || !input || typeof input !== 'object') return {}
  let result = { ...(input || {}) }
  if (result.$ref && !seen.has(result.$ref)) {
    const nextSeen = new Set(seen)
    nextSeen.add(result.$ref)
    result = mergeSchema(materialize(resolvePointer(result.$ref), value, nextSeen), result)
    delete result.$ref
  }
  for (const clause of result.allOf || []) {
    const selected = clause.if
      ? (matchesCondition(clause.if, value) ? clause.then : clause.else)
      : clause
    if (selected) result = mergeSchema(result, materialize(selected, value, seen))
  }
	for (const variants of [result.oneOf, result.anyOf]) {
	  if (!Array.isArray(variants) || variants.length === 0) continue
	  const selected = [...variants].sort((left, right) => variantScore(right, value) - variantScore(left, value))[0]
	  result = mergeSchema(result, materialize(selected, value, seen))
	}
	delete result.allOf
	delete result.oneOf
	delete result.anyOf
  return result
}

const effectiveSchema = computed(() => materialize(props.schema, props.modelValue))
const effectiveUISchema = computed(() => props.uiSchema && typeof props.uiSchema === 'object' ? props.uiSchema : {})
const widget = computed(() => effectiveUISchema.value['ui:widget'] || '')
const fieldDisabled = computed(() => Boolean(effectiveSchema.value.readOnly || effectiveSchema.value.const !== undefined || effectiveUISchema.value['ui:readonly']))
const fieldHelp = computed(() => props.helpOverrides[props.path] || effectiveUISchema.value['ui:help'] || effectiveSchema.value.description || '')
const kind = computed(() => {
  const type = effectiveSchema.value.type
  if (Array.isArray(type)) return type.find(candidate => candidate !== 'null' && valueMatchesType(props.modelValue, candidate)) || type.find(candidate => candidate !== 'null') || 'string'
  return type || (effectiveSchema.value.properties || objectValue.value && Object.keys(objectValue.value).length ? 'object' : 'string')
})
const objectValue = computed(() => props.modelValue && typeof props.modelValue === 'object' && !Array.isArray(props.modelValue) ? props.modelValue : {})
const arrayValue = computed(() => Array.isArray(props.modelValue) ? props.modelValue : [])
const requiredProperties = computed(() => effectiveSchema.value.required || [])
function inferSchema (value) {
  if (Array.isArray(value)) return { type: 'array', items: value.length ? inferSchema(value[0]) : {} }
  if (value !== null && typeof value === 'object') return { type: 'object', additionalProperties: true }
  if (typeof value === 'boolean') return { type: 'boolean' }
  if (typeof value === 'number') return { type: Number.isInteger(value) ? 'integer' : 'number' }
  return { type: 'string' }
}
const propertyEntries = computed(() => {
  const declared = effectiveSchema.value.properties || {}
  const available = [...new Set([...Object.keys(declared), ...Object.keys(objectValue.value)])]
  const requestedOrder = Array.isArray(effectiveUISchema.value['ui:order']) ? effectiveUISchema.value['ui:order'] : []
  const ordered = requestedOrder.flatMap(name => name === '*' ? available : [name]).filter((name, index, names) => available.includes(name) && names.indexOf(name) === index)
  const names = [...ordered, ...available.filter(name => !ordered.includes(name))]
  return names.filter(name => declared[name] !== false).map(name => {
    const additional = effectiveSchema.value.additionalProperties
    const fallback = additional && typeof additional === 'object' ? additional : inferSchema(objectValue.value[name])
    const escaped = name.replaceAll('~', '~0').replaceAll('/', '~1')
    return { name, schema: declared[name] || fallback, uiSchema: effectiveUISchema.value[name] || {}, path: `${props.path}/${escaped}` }
  })
})
const jsonValue = computed(() => JSON.stringify(objectValue.value, null, 2))

function defaultValue (schema) {
  const resolved = materialize(schema, undefined)
  if (resolved.default !== undefined) return JSON.parse(JSON.stringify(resolved.default))
  if (resolved.const !== undefined) return JSON.parse(JSON.stringify(resolved.const))
  if (resolved.type === 'object' || resolved.properties) return {}
  if (resolved.type === 'array') return []
  if (resolved.type === 'boolean') return false
  if (resolved.type === 'integer' || resolved.type === 'number') return resolved.minimum || 0
  if (resolved.enum?.length) return resolved.enum[0]
  return ''
}

const updateScalar = value => emit('update:modelValue', value)

function updateProperty (name, value) {
  emit('update:modelValue', { ...objectValue.value, [name]: value })
}

function updateArrayItem (index, value) {
  const next = [...arrayValue.value]
  next[index] = value
  emit('update:modelValue', next)
}

function addArrayItem () {
  emit('update:modelValue', [...arrayValue.value, defaultValue(effectiveSchema.value.items || {})])
}

function removeArrayItem (index) {
  emit('update:modelValue', arrayValue.value.filter((_, itemIndex) => itemIndex !== index))
}

function updateJSON (value) {
  try {
    emit('update:modelValue', JSON.parse(value))
  } catch (_) {
    // Authoritative Agent validation remains the final gate; keep the last valid form value.
  }
}
</script>

<style scoped lang="scss">
.schema-field { margin-bottom: 12px; }
.object-fields { border-left: 2px solid var(--el-border-color); padding-left: 16px; }
.array-item { margin-bottom: 10px; }
.required { color: var(--el-color-danger); }
.description { color: var(--el-text-color-secondary); font-size: 12px; margin: 4px 0 8px; }
</style>
