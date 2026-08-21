<template>
  <section class="schema-field">
    <template v-if="kind === 'object'">
      <h4 v-if="label">{{ label }} <span v-if="required" class="required">*</span></h4>
      <p v-if="effectiveSchema.description" class="description">{{ effectiveSchema.description }}</p>
      <div v-if="propertyEntries.length" class="object-fields">
        <SchemaField
          v-for="entry in propertyEntries"
          :key="entry.name"
          :schema="entry.schema"
          :root-schema="rootSchema"
          :model-value="objectValue[entry.name]"
          :label="entry.schema.title || entry.name"
          :required="requiredProperties.includes(entry.name)"
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
      <p v-if="effectiveSchema.description" class="description">{{ effectiveSchema.description }}</p>
      <el-card v-for="(item, index) in arrayValue" :key="index" class="array-item" shadow="never">
        <SchemaField
          :schema="effectiveSchema.items || {}"
          :root-schema="rootSchema"
          :model-value="item"
          :label="`${label || 'item'} ${index + 1}`"
          @update:model-value="updateArrayItem(index, $event)"
        />
        <el-button type="danger" plain size="small" @click="removeArrayItem(index)">Remove</el-button>
      </el-card>
      <el-button plain size="small" @click="addArrayItem">Add {{ label || 'item' }}</el-button>
    </template>

    <el-form-item v-else :label="label" :required="required">
      <el-select
        v-if="effectiveSchema.enum"
        :model-value="modelValue"
        clearable
        @update:model-value="updateScalar"
      >
        <el-option v-for="option in effectiveSchema.enum" :key="String(option)" :label="String(option)" :value="option" />
      </el-select>
      <el-switch
        v-else-if="kind === 'boolean'"
        :model-value="Boolean(modelValue)"
        @update:model-value="updateScalar"
      />
      <el-input-number
        v-else-if="kind === 'integer' || kind === 'number'"
        :model-value="modelValue"
        :min="effectiveSchema.minimum"
        :max="effectiveSchema.maximum"
        @update:model-value="updateScalar"
      />
      <el-input
        v-else
        :model-value="modelValue == null ? '' : String(modelValue)"
        :disabled="effectiveSchema.const !== undefined"
        @update:model-value="updateScalar"
      />
      <div v-if="effectiveSchema.description" class="description">{{ effectiveSchema.description }}</div>
    </el-form-item>
  </section>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  schema: { type: Object, default: () => ({}) },
  rootSchema: { type: Object, required: true },
  modelValue: { default: undefined },
  label: { type: String, default: '' },
  required: { type: Boolean, default: false },
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
    return true
  })
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
  return result
}

const effectiveSchema = computed(() => materialize(props.schema, props.modelValue))
const kind = computed(() => effectiveSchema.value.type || (effectiveSchema.value.properties ? 'object' : 'string'))
const objectValue = computed(() => props.modelValue && typeof props.modelValue === 'object' && !Array.isArray(props.modelValue) ? props.modelValue : {})
const arrayValue = computed(() => Array.isArray(props.modelValue) ? props.modelValue : [])
const requiredProperties = computed(() => effectiveSchema.value.required || [])
const propertyEntries = computed(() => Object.entries(effectiveSchema.value.properties || {})
  .filter(([, schema]) => schema !== false)
  .map(([name, schema]) => ({ name, schema })))
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
