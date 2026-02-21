/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import MonacoEditor from '@monaco-editor/react';
import { useTranslation } from 'react-i18next';
import {
  Banner,
  Button,
  Card,
  Col,
  Input,
  Modal,
  Row,
  Select,
  Space,
  Switch,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { IconDelete, IconPlus } from '@douyinfe/semi-icons';
import { showError, verifyJSON } from '../../../../helpers';
import JSONEditor from '../../../common/ui/JSONEditor';

const { Text } = Typography;

const OPERATION_MODE_OPTIONS = [
  { label: 'JSON · set', value: 'set' },
  { label: 'JSON · delete', value: 'delete' },
  { label: 'JSON · append', value: 'append' },
  { label: 'JSON · prepend', value: 'prepend' },
  { label: 'JSON · copy', value: 'copy' },
  { label: 'JSON · move', value: 'move' },
  { label: 'JSON · replace', value: 'replace' },
  { label: 'JSON · regex_replace', value: 'regex_replace' },
  { label: 'JSON · trim_prefix', value: 'trim_prefix' },
  { label: 'JSON · trim_suffix', value: 'trim_suffix' },
  { label: 'JSON · ensure_prefix', value: 'ensure_prefix' },
  { label: 'JSON · ensure_suffix', value: 'ensure_suffix' },
  { label: 'JSON · trim_space', value: 'trim_space' },
  { label: 'JSON · to_lower', value: 'to_lower' },
  { label: 'JSON · to_upper', value: 'to_upper' },
  { label: 'Control · return_error', value: 'return_error' },
  { label: 'Control · prune_objects', value: 'prune_objects' },
  { label: 'Header · set_header', value: 'set_header' },
  { label: 'Header · delete_header', value: 'delete_header' },
  { label: 'Header · copy_header', value: 'copy_header' },
  { label: 'Header · move_header', value: 'move_header' },
];

const OPERATION_MODE_VALUES = new Set(
  OPERATION_MODE_OPTIONS.map((item) => item.value),
);

const CONDITION_MODE_OPTIONS = [
  { label: 'full', value: 'full' },
  { label: 'prefix', value: 'prefix' },
  { label: 'suffix', value: 'suffix' },
  { label: 'contains', value: 'contains' },
  { label: 'gt', value: 'gt' },
  { label: 'gte', value: 'gte' },
  { label: 'lt', value: 'lt' },
  { label: 'lte', value: 'lte' },
];

const CONDITION_MODE_VALUES = new Set(
  CONDITION_MODE_OPTIONS.map((item) => item.value),
);

const MODE_META = {
  delete: { path: true },
  set: { path: true, value: true, keepOrigin: true },
  append: { path: true, value: true, keepOrigin: true },
  prepend: { path: true, value: true, keepOrigin: true },
  copy: { from: true, to: true },
  move: { from: true, to: true },
  replace: { path: true, from: true, to: false },
  regex_replace: { path: true, from: true, to: false },
  trim_prefix: { path: true, value: true },
  trim_suffix: { path: true, value: true },
  ensure_prefix: { path: true, value: true },
  ensure_suffix: { path: true, value: true },
  trim_space: { path: true },
  to_lower: { path: true },
  to_upper: { path: true },
  return_error: { value: true },
  prune_objects: { pathOptional: true, value: true },
  set_header: { path: true, value: true, keepOrigin: true },
  delete_header: { path: true },
  copy_header: { from: true, to: true, keepOrigin: true, pathAlias: true },
  move_header: { from: true, to: true, keepOrigin: true, pathAlias: true },
};

const VALUE_REQUIRED_MODES = new Set([
  'trim_prefix',
  'trim_suffix',
  'ensure_prefix',
  'ensure_suffix',
  'set_header',
  'return_error',
  'prune_objects',
]);

const FROM_REQUIRED_MODES = new Set([
  'copy',
  'move',
  'replace',
  'regex_replace',
  'copy_header',
  'move_header',
]);

const TO_REQUIRED_MODES = new Set(['copy', 'move', 'copy_header', 'move_header']);

const MODE_DESCRIPTIONS = {
  set: 'Set JSON value at path',
  delete: 'Delete JSON field at path',
  append: 'Append value to array/string/object',
  prepend: 'Prepend value to array/string/object',
  copy: 'Copy JSON value from from -> to',
  move: 'Move JSON value from from -> to',
  replace: 'String replace on target path',
  regex_replace: 'Regex replace on target path',
  trim_prefix: 'Trim prefix on string value',
  trim_suffix: 'Trim suffix on string value',
  ensure_prefix: 'Ensure string starts with prefix',
  ensure_suffix: 'Ensure string ends with suffix',
  trim_space: 'Trim spaces/newlines on string value',
  to_lower: 'Convert string to lower case',
  to_upper: 'Convert string to upper case',
  return_error: 'Stop processing and return custom error',
  prune_objects: 'Remove objects matching conditions',
  set_header: 'Set runtime override header',
  delete_header: 'Delete runtime override header',
  copy_header: 'Copy header from from -> to',
  move_header: 'Move header from from -> to',
};

const OPERATION_PATH_SUGGESTIONS = [
  'model',
  'temperature',
  'max_tokens',
  'messages.-1.content',
  'metadata.conversation_id',
];

const CONDITION_PATH_SUGGESTIONS = [
  'model',
  'retry.is_retry',
  'last_error.code',
  'request_headers.authorization',
  'header_override_normalized.x_debug_mode',
];

const LEGACY_TEMPLATE = {
  temperature: 0,
  max_tokens: 1000,
};

const OPERATION_TEMPLATE = {
  operations: [
    {
      path: 'temperature',
      mode: 'set',
      value: 0.7,
      conditions: [
        {
          path: 'model',
          mode: 'prefix',
          value: 'gpt',
        },
      ],
      logic: 'AND',
    },
  ],
};

const MONACO_SCHEMA_URI = 'https://new-api.local/schemas/param-override.schema.json';
const MONACO_MODEL_URI = 'inmemory://new-api/param-override.json';

const JSON_SCALAR_SCHEMA = {
  oneOf: [
    { type: 'string' },
    { type: 'number' },
    { type: 'boolean' },
    { type: 'null' },
    { type: 'array' },
    { type: 'object' },
  ],
};

const PARAM_OVERRIDE_JSON_SCHEMA = {
  $schema: 'http://json-schema.org/draft-07/schema#',
  type: 'object',
  properties: {
    operations: {
      type: 'array',
      description: 'Operation pipeline for new param override format.',
      items: {
        type: 'object',
        properties: {
          mode: {
            type: 'string',
            enum: OPERATION_MODE_OPTIONS.map((item) => item.value),
          },
          path: { type: 'string' },
          from: { type: 'string' },
          to: { type: 'string' },
          keep_origin: { type: 'boolean' },
          value: JSON_SCALAR_SCHEMA,
          logic: { type: 'string', enum: ['AND', 'OR'] },
          conditions: {
            oneOf: [
              {
                type: 'array',
                items: {
                  type: 'object',
                  properties: {
                    path: { type: 'string' },
                    mode: {
                      type: 'string',
                      enum: CONDITION_MODE_OPTIONS.map((item) => item.value),
                    },
                    value: JSON_SCALAR_SCHEMA,
                    invert: { type: 'boolean' },
                    pass_missing_key: { type: 'boolean' },
                  },
                  required: ['path', 'mode'],
                  additionalProperties: false,
                },
              },
              {
                type: 'object',
                additionalProperties: JSON_SCALAR_SCHEMA,
              },
            ],
          },
        },
        required: ['mode'],
        additionalProperties: false,
        allOf: [
          {
            if: { properties: { mode: { const: 'set' } }, required: ['mode'] },
            then: { required: ['path'] },
          },
          {
            if: { properties: { mode: { const: 'delete' } }, required: ['mode'] },
            then: { required: ['path'] },
          },
          {
            if: { properties: { mode: { const: 'append' } }, required: ['mode'] },
            then: { required: ['path'] },
          },
          {
            if: { properties: { mode: { const: 'prepend' } }, required: ['mode'] },
            then: { required: ['path'] },
          },
          {
            if: { properties: { mode: { const: 'copy' } }, required: ['mode'] },
            then: { required: ['from', 'to'] },
          },
          {
            if: { properties: { mode: { const: 'move' } }, required: ['mode'] },
            then: { required: ['from', 'to'] },
          },
          {
            if: { properties: { mode: { const: 'replace' } }, required: ['mode'] },
            then: { required: ['path', 'from'] },
          },
          {
            if: {
              properties: { mode: { const: 'regex_replace' } },
              required: ['mode'],
            },
            then: { required: ['path', 'from'] },
          },
          {
            if: {
              properties: { mode: { const: 'trim_prefix' } },
              required: ['mode'],
            },
            then: { required: ['path', 'value'] },
          },
          {
            if: {
              properties: { mode: { const: 'trim_suffix' } },
              required: ['mode'],
            },
            then: { required: ['path', 'value'] },
          },
          {
            if: {
              properties: { mode: { const: 'ensure_prefix' } },
              required: ['mode'],
            },
            then: { required: ['path', 'value'] },
          },
          {
            if: {
              properties: { mode: { const: 'ensure_suffix' } },
              required: ['mode'],
            },
            then: { required: ['path', 'value'] },
          },
          {
            if: {
              properties: { mode: { const: 'trim_space' } },
              required: ['mode'],
            },
            then: { required: ['path'] },
          },
          {
            if: {
              properties: { mode: { const: 'to_lower' } },
              required: ['mode'],
            },
            then: { required: ['path'] },
          },
          {
            if: {
              properties: { mode: { const: 'to_upper' } },
              required: ['mode'],
            },
            then: { required: ['path'] },
          },
          {
            if: {
              properties: { mode: { const: 'return_error' } },
              required: ['mode'],
            },
            then: { required: ['value'] },
          },
          {
            if: {
              properties: { mode: { const: 'prune_objects' } },
              required: ['mode'],
            },
            then: { required: ['value'] },
          },
          {
            if: {
              properties: { mode: { const: 'set_header' } },
              required: ['mode'],
            },
            then: { required: ['path', 'value'] },
          },
          {
            if: {
              properties: { mode: { const: 'delete_header' } },
              required: ['mode'],
            },
            then: { required: ['path'] },
          },
          {
            if: {
              properties: { mode: { const: 'copy_header' } },
              required: ['mode'],
            },
            then: {
              anyOf: [{ required: ['path'] }, { required: ['from', 'to'] }],
            },
          },
          {
            if: {
              properties: { mode: { const: 'move_header' } },
              required: ['mode'],
            },
            then: {
              anyOf: [{ required: ['path'] }, { required: ['from', 'to'] }],
            },
          },
        ],
      },
    },
  },
  additionalProperties: true,
};

let localIdSeed = 0;
const nextLocalId = () => `param_override_${Date.now()}_${localIdSeed++}`;

const toValueText = (value) => {
  if (value === undefined) return '';
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value);
  } catch (error) {
    return String(value);
  }
};

const parseLooseValue = (valueText) => {
  const raw = String(valueText ?? '');
  if (raw.trim() === '') return '';
  try {
    return JSON.parse(raw);
  } catch (error) {
    return raw;
  }
};

const normalizeCondition = (condition = {}) => ({
  id: nextLocalId(),
  path: typeof condition.path === 'string' ? condition.path : '',
  mode: CONDITION_MODE_VALUES.has(condition.mode) ? condition.mode : 'full',
  value_text: toValueText(condition.value),
  invert: condition.invert === true,
  pass_missing_key: condition.pass_missing_key === true,
});

const createDefaultCondition = () => normalizeCondition({});

const normalizeOperation = (operation = {}) => ({
  id: nextLocalId(),
  path: typeof operation.path === 'string' ? operation.path : '',
  mode: OPERATION_MODE_VALUES.has(operation.mode) ? operation.mode : 'set',
  value_text: toValueText(operation.value),
  keep_origin: operation.keep_origin === true,
  from: typeof operation.from === 'string' ? operation.from : '',
  to: typeof operation.to === 'string' ? operation.to : '',
  logic: String(operation.logic || 'OR').toUpperCase() === 'AND' ? 'AND' : 'OR',
  conditions: Array.isArray(operation.conditions)
    ? operation.conditions.map(normalizeCondition)
    : [],
});

const createDefaultOperation = () => normalizeOperation({ mode: 'set' });

const parseInitialState = (rawValue) => {
  const text = typeof rawValue === 'string' ? rawValue : '';
  const trimmed = text.trim();
  if (!trimmed) {
    return {
      editMode: 'visual',
      visualMode: 'operations',
      legacyValue: '',
      operations: [createDefaultOperation()],
      jsonText: '',
      jsonError: '',
    };
  }

  if (!verifyJSON(trimmed)) {
    return {
      editMode: 'json',
      visualMode: 'operations',
      legacyValue: '',
      operations: [createDefaultOperation()],
      jsonText: text,
      jsonError: 'JSON format is invalid',
    };
  }

  const parsed = JSON.parse(trimmed);
  const pretty = JSON.stringify(parsed, null, 2);

  if (
    parsed &&
    typeof parsed === 'object' &&
    !Array.isArray(parsed) &&
    Array.isArray(parsed.operations)
  ) {
    return {
      editMode: 'visual',
      visualMode: 'operations',
      legacyValue: '',
      operations:
        parsed.operations.length > 0
          ? parsed.operations.map(normalizeOperation)
          : [createDefaultOperation()],
      jsonText: pretty,
      jsonError: '',
    };
  }

  if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
    return {
      editMode: 'visual',
      visualMode: 'legacy',
      legacyValue: pretty,
      operations: [createDefaultOperation()],
      jsonText: pretty,
      jsonError: '',
    };
  }

  return {
    editMode: 'json',
    visualMode: 'operations',
    legacyValue: '',
    operations: [createDefaultOperation()],
    jsonText: pretty,
    jsonError: '',
  };
};

const isOperationBlank = (operation) => {
  const hasCondition = (operation.conditions || []).some(
    (condition) =>
      condition.path.trim() ||
      String(condition.value_text ?? '').trim() ||
      condition.mode !== 'full' ||
      condition.invert ||
      condition.pass_missing_key,
  );
  return (
    operation.mode === 'set' &&
    !operation.path.trim() &&
    !operation.from.trim() &&
    !operation.to.trim() &&
    String(operation.value_text ?? '').trim() === '' &&
    !operation.keep_origin &&
    !hasCondition
  );
};

const buildConditionPayload = (condition) => {
  const path = condition.path.trim();
  if (!path) return null;
  const payload = {
    path,
    mode: condition.mode || 'full',
    value: parseLooseValue(condition.value_text),
  };
  if (condition.invert) payload.invert = true;
  if (condition.pass_missing_key) payload.pass_missing_key = true;
  return payload;
};

const validateOperations = (operations, t) => {
  for (let i = 0; i < operations.length; i++) {
    const op = operations[i];
    const mode = op.mode || 'set';
    const meta = MODE_META[mode] || MODE_META.set;
    const line = i + 1;
    const pathValue = op.path.trim();
    const fromValue = op.from.trim();
    const toValue = op.to.trim();

    if (meta.path && !pathValue) {
      return t('第 {{line}} 条操作缺少 path', { line });
    }
    if (FROM_REQUIRED_MODES.has(mode) && !fromValue) {
      if (!(meta.pathAlias && pathValue)) {
        return t('第 {{line}} 条操作缺少 from', { line });
      }
    }
    if (TO_REQUIRED_MODES.has(mode) && !toValue) {
      if (!(meta.pathAlias && pathValue)) {
        return t('第 {{line}} 条操作缺少 to', { line });
      }
    }
    if (meta.from && !fromValue) {
      return t('第 {{line}} 条操作缺少 from', { line });
    }
    if (meta.to && !toValue) {
      return t('第 {{line}} 条操作缺少 to', { line });
    }
    if (
      VALUE_REQUIRED_MODES.has(mode) &&
      String(op.value_text ?? '').trim() === ''
    ) {
      return t('第 {{line}} 条操作缺少 value', { line });
    }
    if (mode === 'return_error') {
      const raw = String(op.value_text ?? '').trim();
      if (!raw) {
        return t('第 {{line}} 条操作缺少 value', { line });
      }
      try {
        const parsed = JSON.parse(raw);
        if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
          if (!String(parsed.message || '').trim()) {
            return t('第 {{line}} 条 return_error 需要 message', { line });
          }
        }
      } catch (error) {
        // plain string value is allowed
      }
    }
  }
  return '';
};

const ParamOverrideEditorModal = ({ visible, value, onSave, onCancel }) => {
  const { t } = useTranslation();

  const [editMode, setEditMode] = useState('visual');
  const [visualMode, setVisualMode] = useState('operations');
  const [legacyValue, setLegacyValue] = useState('');
  const [operations, setOperations] = useState([createDefaultOperation()]);
  const [jsonText, setJsonText] = useState('');
  const [jsonError, setJsonError] = useState('');
  const monacoConfiguredRef = useRef(false);

  const configureMonaco = useCallback((monaco) => {
    if (monacoConfiguredRef.current) return;
    monaco.languages.json.jsonDefaults.setDiagnosticsOptions({
      validate: true,
      allowComments: false,
      enableSchemaRequest: false,
      schemas: [
        {
          uri: MONACO_SCHEMA_URI,
          fileMatch: [MONACO_MODEL_URI, '*param-override*.json'],
          schema: PARAM_OVERRIDE_JSON_SCHEMA,
        },
      ],
    });
    monacoConfiguredRef.current = true;
  }, []);

  useEffect(() => {
    if (!visible) return;
    const nextState = parseInitialState(value);
    setEditMode(nextState.editMode);
    setVisualMode(nextState.visualMode);
    setLegacyValue(nextState.legacyValue);
    setOperations(nextState.operations);
    setJsonText(nextState.jsonText);
    setJsonError(nextState.jsonError);
  }, [visible, value]);

  const operationCount = useMemo(
    () => operations.filter((item) => !isOperationBlank(item)).length,
    [operations],
  );

  const buildVisualJson = useCallback(() => {
    if (visualMode === 'legacy') {
      const trimmed = legacyValue.trim();
      if (!trimmed) return '';
      if (!verifyJSON(trimmed)) {
        throw new Error(t('参数覆盖必须是合法的 JSON 格式！'));
      }
      const parsed = JSON.parse(trimmed);
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        throw new Error(t('旧格式必须是 JSON 对象'));
      }
      return JSON.stringify(parsed, null, 2);
    }

    const filteredOps = operations.filter((item) => !isOperationBlank(item));
    if (filteredOps.length === 0) return '';

    const message = validateOperations(filteredOps, t);
    if (message) {
      throw new Error(message);
    }

    const payloadOps = filteredOps.map((operation) => {
      const mode = operation.mode || 'set';
      const meta = MODE_META[mode] || MODE_META.set;
      const pathValue = operation.path.trim();
      const fromValue = operation.from.trim();
      const toValue = operation.to.trim();
      const payload = { mode };
      if (meta.path) {
        payload.path = pathValue;
      }
      if (meta.pathOptional && pathValue) {
        payload.path = pathValue;
      }
      if (meta.value) {
        payload.value = parseLooseValue(operation.value_text);
      }
      if (meta.keepOrigin && operation.keep_origin) {
        payload.keep_origin = true;
      }
      if (meta.from) {
        payload.from = fromValue;
      }
      if (!meta.to && operation.to.trim()) {
        payload.to = toValue;
      }
      if (meta.to) {
        payload.to = toValue;
      }
      if (meta.pathAlias) {
        if (!payload.from && pathValue) {
          payload.from = pathValue;
        }
        if (!payload.to && pathValue) {
          payload.to = pathValue;
        }
      }

      const conditions = (operation.conditions || [])
        .map(buildConditionPayload)
        .filter(Boolean);

      if (conditions.length > 0) {
        payload.conditions = conditions;
        payload.logic = operation.logic === 'AND' ? 'AND' : 'OR';
      }

      return payload;
    });

    return JSON.stringify({ operations: payloadOps }, null, 2);
  }, [legacyValue, operations, t, visualMode]);

  const switchToJsonMode = () => {
    if (editMode === 'json') return;
    try {
      setJsonText(buildVisualJson());
      setJsonError('');
      setEditMode('json');
    } catch (error) {
      showError(error.message);
    }
  };

  const switchToVisualMode = () => {
    if (editMode === 'visual') return;
    const trimmed = jsonText.trim();
    if (!trimmed) {
      setVisualMode('operations');
      setOperations([createDefaultOperation()]);
      setLegacyValue('');
      setJsonError('');
      setEditMode('visual');
      return;
    }
    if (!verifyJSON(trimmed)) {
      showError(t('参数覆盖必须是合法的 JSON 格式！'));
      return;
    }
    const parsed = JSON.parse(trimmed);
    if (
      parsed &&
      typeof parsed === 'object' &&
      !Array.isArray(parsed) &&
      Array.isArray(parsed.operations)
    ) {
      setVisualMode('operations');
      setOperations(
        parsed.operations.length > 0
          ? parsed.operations.map(normalizeOperation)
          : [createDefaultOperation()],
      );
      setLegacyValue('');
      setJsonError('');
      setEditMode('visual');
      return;
    }
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      setVisualMode('legacy');
      setLegacyValue(JSON.stringify(parsed, null, 2));
      setOperations([createDefaultOperation()]);
      setJsonError('');
      setEditMode('visual');
      return;
    }
    showError(t('参数覆盖必须是合法的 JSON 对象'));
  };

  const setOldTemplate = () => {
    const text = JSON.stringify(LEGACY_TEMPLATE, null, 2);
    setVisualMode('legacy');
    setLegacyValue(text);
    setJsonText(text);
    setJsonError('');
    setEditMode('visual');
  };

  const setNewTemplate = () => {
    setVisualMode('operations');
    setOperations(OPERATION_TEMPLATE.operations.map(normalizeOperation));
    setJsonText(JSON.stringify(OPERATION_TEMPLATE, null, 2));
    setJsonError('');
    setEditMode('visual');
  };

  const clearValue = () => {
    setVisualMode('operations');
    setLegacyValue('');
    setOperations([createDefaultOperation()]);
    setJsonText('');
    setJsonError('');
  };

  const updateOperation = (operationId, patch) => {
    setOperations((prev) =>
      prev.map((item) =>
        item.id === operationId ? { ...item, ...patch } : item,
      ),
    );
  };

  const addOperation = () => {
    setOperations((prev) => [...prev, createDefaultOperation()]);
  };

  const duplicateOperation = (operationId) => {
    setOperations((prev) => {
      const index = prev.findIndex((item) => item.id === operationId);
      if (index < 0) return prev;
      const source = prev[index];
      const cloned = normalizeOperation({
        path: source.path,
        mode: source.mode,
        value: parseLooseValue(source.value_text),
        keep_origin: source.keep_origin,
        from: source.from,
        to: source.to,
        logic: source.logic,
        conditions: (source.conditions || []).map((condition) => ({
          path: condition.path,
          mode: condition.mode,
          value: parseLooseValue(condition.value_text),
          invert: condition.invert,
          pass_missing_key: condition.pass_missing_key,
        })),
      });
      const next = [...prev];
      next.splice(index + 1, 0, cloned);
      return next;
    });
  };

  const removeOperation = (operationId) => {
    setOperations((prev) => {
      if (prev.length <= 1) return [createDefaultOperation()];
      return prev.filter((item) => item.id !== operationId);
    });
  };

  const addCondition = (operationId) => {
    setOperations((prev) =>
      prev.map((operation) =>
        operation.id === operationId
          ? {
              ...operation,
              conditions: [...(operation.conditions || []), createDefaultCondition()],
            }
          : operation,
      ),
    );
  };

  const updateCondition = (operationId, conditionId, patch) => {
    setOperations((prev) =>
      prev.map((operation) => {
        if (operation.id !== operationId) return operation;
        return {
          ...operation,
          conditions: (operation.conditions || []).map((condition) =>
            condition.id === conditionId
              ? { ...condition, ...patch }
              : condition,
          ),
        };
      }),
    );
  };

  const removeCondition = (operationId, conditionId) => {
    setOperations((prev) =>
      prev.map((operation) => {
        if (operation.id !== operationId) return operation;
        return {
          ...operation,
          conditions: (operation.conditions || []).filter(
            (condition) => condition.id !== conditionId,
          ),
        };
      }),
    );
  };

  const handleJsonChange = (nextValue) => {
    setJsonText(nextValue);
    const trimmed = String(nextValue || '').trim();
    if (!trimmed) {
      setJsonError('');
      return;
    }
    if (!verifyJSON(trimmed)) {
      setJsonError(t('JSON格式错误'));
      return;
    }
    setJsonError('');
  };

  const formatJson = () => {
    const trimmed = jsonText.trim();
    if (!trimmed) return;
    if (!verifyJSON(trimmed)) {
      showError(t('参数覆盖必须是合法的 JSON 格式！'));
      return;
    }
    setJsonText(JSON.stringify(JSON.parse(trimmed), null, 2));
    setJsonError('');
  };

  const visualPreview = useMemo(() => {
    if (editMode !== 'visual' || visualMode !== 'operations') {
      return '';
    }
    try {
      return buildVisualJson() || '';
    } catch (error) {
      return `// ${error.message}`;
    }
  }, [buildVisualJson, editMode, visualMode]);

  const handleSave = () => {
    try {
      let result = '';
      if (editMode === 'json') {
        const trimmed = jsonText.trim();
        if (!trimmed) {
          result = '';
        } else {
          if (!verifyJSON(trimmed)) {
            throw new Error(t('参数覆盖必须是合法的 JSON 格式！'));
          }
          result = JSON.stringify(JSON.parse(trimmed), null, 2);
        }
      } else {
        result = buildVisualJson();
      }
      onSave?.(result);
    } catch (error) {
      showError(error.message);
    }
  };

  return (
    <Modal
      title={t('参数覆盖')}
      visible={visible}
      width={980}
      onCancel={onCancel}
      onOk={handleSave}
      okText={t('保存')}
      cancelText={t('取消')}
    >
      <Space vertical align='start' spacing={12} style={{ width: '100%' }}>
        <Banner
          fullMode={false}
          type='info'
          description={t(
            '支持旧格式直接覆盖，也支持新格式 operations 条件编辑；可在可视化和 JSON 之间双向切换。',
          )}
        />

        <Space wrap>
          <Button
            type={editMode === 'visual' ? 'primary' : 'tertiary'}
            onClick={switchToVisualMode}
          >
            {t('可视化')}
          </Button>
          <Button
            type={editMode === 'json' ? 'primary' : 'tertiary'}
            onClick={switchToJsonMode}
          >
            {t('JSON 模式')}
          </Button>
          <Button onClick={setOldTemplate}>{t('旧格式模板')}</Button>
          <Button onClick={setNewTemplate}>{t('新格式模板')}</Button>
          <Button onClick={clearValue}>{t('不更改')}</Button>
        </Space>

        {editMode === 'visual' ? (
          <div style={{ width: '100%' }}>
            <Space wrap style={{ marginBottom: 12 }}>
              <Button
                type={visualMode === 'operations' ? 'primary' : 'tertiary'}
                onClick={() => setVisualMode('operations')}
              >
                {t('新格式模板')}
              </Button>
              <Button
                type={visualMode === 'legacy' ? 'primary' : 'tertiary'}
                onClick={() => setVisualMode('legacy')}
              >
                {t('旧格式模板')}
              </Button>
            </Space>

            {visualMode === 'legacy' ? (
              <JSONEditor
                field='param_override_legacy'
                label={t('旧格式（直接覆盖）：')}
                placeholder={JSON.stringify(LEGACY_TEMPLATE, null, 2)}
                value={legacyValue}
                onChange={setLegacyValue}
                template={LEGACY_TEMPLATE}
                templateLabel={t('填入模板')}
                editorType='keyValue'
                showClear
              />
            ) : (
              <div>
                <div className='flex items-center justify-between mb-3'>
                  <Space>
                    <Text>{t('新格式（支持条件判断与json自定义）：')}</Text>
                    <Tag color='cyan'>
                      {`${t('规则')}: ${operationCount}`}
                    </Tag>
                  </Space>
                  <Button icon={<IconPlus />} onClick={addOperation}>
                    {t('新增规则')}
                  </Button>
                </div>

                <Space vertical spacing={8} style={{ width: '100%' }}>
                  {operations.map((operation, index) => {
                    const mode = operation.mode || 'set';
                    const meta = MODE_META[mode] || MODE_META.set;
                    const conditions = operation.conditions || [];
                    return (
                      <Card key={operation.id} className='!rounded-xl border'>
                        <div className='flex items-center justify-between mb-2'>
                          <Space>
                            <Tag>{`#${index + 1}`}</Tag>
                            <Text>{mode}</Text>
                          </Space>
                          <Space>
                            <Button
                              size='small'
                              type='tertiary'
                              onClick={() => duplicateOperation(operation.id)}
                            >
                              {t('复制')}
                            </Button>
                            <Button
                              theme='borderless'
                              type='danger'
                              icon={<IconDelete />}
                              onClick={() => removeOperation(operation.id)}
                            />
                          </Space>
                        </div>

                        <Row gutter={12}>
                          <Col xs={24} md={8}>
                            <Text type='tertiary' size='small'>
                              mode
                            </Text>
                            <Select
                              value={mode}
                              optionList={OPERATION_MODE_OPTIONS}
                              onChange={(nextMode) =>
                                updateOperation(operation.id, { mode: nextMode })
                              }
                              style={{ width: '100%' }}
                            />
                          </Col>
                          {meta.path || meta.pathOptional ? (
                            <Col xs={24} md={16}>
                              <Text type='tertiary' size='small'>
                                {meta.pathOptional ? 'path (optional)' : 'path'}
                              </Text>
                              <Input
                                value={operation.path}
                                placeholder={
                                  mode.includes('header')
                                    ? 'X-Debug-Mode'
                                    : mode === 'prune_objects'
                                      ? 'messages (optional)'
                                      : 'temperature'
                                }
                                onChange={(nextValue) =>
                                  updateOperation(operation.id, {
                                    path: nextValue,
                                  })
                                }
                              />
                              <Space wrap style={{ marginTop: 6 }}>
                                {OPERATION_PATH_SUGGESTIONS.map((pathItem) => (
                                  <Tag
                                    key={`${operation.id}_${pathItem}`}
                                    size='small'
                                    color='grey'
                                    className='cursor-pointer'
                                    onClick={() =>
                                      updateOperation(operation.id, {
                                        path: pathItem,
                                      })
                                    }
                                  >
                                    {pathItem}
                                  </Tag>
                                ))}
                              </Space>
                            </Col>
                          ) : null}
                        </Row>
                        <Text type='tertiary' size='small' className='mt-1 block'>
                          {MODE_DESCRIPTIONS[mode] || ''}
                        </Text>

                        {meta.value ? (
                          <div className='mt-2'>
                            <Text type='tertiary' size='small'>
                              value (JSON or plain text)
                            </Text>
                            <Input.TextArea
                              value={operation.value_text}
                              autosize={{ minRows: 1, maxRows: 4 }}
                              placeholder='0.7'
                              onChange={(nextValue) =>
                                updateOperation(operation.id, {
                                  value_text: nextValue,
                                })
                              }
                            />
                          </div>
                        ) : null}

                        {meta.keepOrigin ? (
                          <div className='mt-2'>
                            <Switch
                              checked={operation.keep_origin}
                              checkedText={t('开')}
                              uncheckedText={t('关')}
                              onChange={(nextValue) =>
                                updateOperation(operation.id, {
                                  keep_origin: nextValue,
                                })
                              }
                            />
                            <Text type='tertiary' size='small' className='ml-2'>
                              keep_origin
                            </Text>
                          </div>
                        ) : null}

                        {meta.from || meta.to === false || meta.to ? (
                          <Row gutter={12} style={{ marginTop: 8 }}>
                            {meta.from || meta.to === false ? (
                              <Col xs={24} md={12}>
                                <Text type='tertiary' size='small'>
                                  from
                                </Text>
                                <Input
                                  value={operation.from}
                                  placeholder={
                                    mode.includes('header')
                                      ? 'Authorization'
                                      : 'model'
                                  }
                                  onChange={(nextValue) =>
                                    updateOperation(operation.id, {
                                      from: nextValue,
                                    })
                                  }
                                />
                              </Col>
                            ) : null}
                            {meta.to || meta.to === false ? (
                              <Col xs={24} md={12}>
                                <Text type='tertiary' size='small'>
                                  to
                                </Text>
                                <Input
                                  value={operation.to}
                                  placeholder={
                                    mode.includes('header')
                                      ? 'X-Upstream-Auth'
                                      : 'original_model'
                                  }
                                  onChange={(nextValue) =>
                                    updateOperation(operation.id, { to: nextValue })
                                  }
                                />
                              </Col>
                            ) : null}
                          </Row>
                        ) : null}

                        <div className='mt-3 border rounded-lg p-2'>
                          <div className='flex items-center justify-between mb-2'>
                            <Space>
                              <Text>{t('条件')}</Text>
                              <Select
                                value={operation.logic || 'OR'}
                                optionList={[
                                  { label: 'OR', value: 'OR' },
                                  { label: 'AND', value: 'AND' },
                                ]}
                                style={{ width: 96 }}
                                onChange={(nextValue) =>
                                  updateOperation(operation.id, {
                                    logic: nextValue,
                                  })
                                }
                              />
                            </Space>
                            <Button
                              icon={<IconPlus />}
                              size='small'
                              onClick={() => addCondition(operation.id)}
                            >
                              {t('新增条件')}
                            </Button>
                          </div>

                          {conditions.length === 0 ? (
                            <Text type='tertiary' size='small'>
                              {t('没有条件时，默认总是执行该操作。')}
                            </Text>
                          ) : (
                            <Space vertical spacing={8} style={{ width: '100%' }}>
                              {conditions.map((condition, conditionIndex) => (
                                <Card
                                  key={condition.id}
                                  bodyStyle={{ padding: 10 }}
                                  className='!rounded-lg'
                                >
                                  <div className='flex items-center justify-between mb-2'>
                                    <Tag size='small'>{`C${conditionIndex + 1}`}</Tag>
                                    <Button
                                      theme='borderless'
                                      type='danger'
                                      icon={<IconDelete />}
                                      size='small'
                                      onClick={() =>
                                        removeCondition(operation.id, condition.id)
                                      }
                                    />
                                  </div>
                                  <Row gutter={12}>
                                    <Col xs={24} md={10}>
                                      <Text type='tertiary' size='small'>
                                        path
                                      </Text>
                                      <Input
                                        value={condition.path}
                                        placeholder='model'
                                        onChange={(nextValue) =>
                                          updateCondition(
                                            operation.id,
                                            condition.id,
                                            { path: nextValue },
                                          )
                                        }
                                      />
                                      <Space wrap style={{ marginTop: 6 }}>
                                        {CONDITION_PATH_SUGGESTIONS.map(
                                          (pathItem) => (
                                            <Tag
                                              key={`${condition.id}_${pathItem}`}
                                              size='small'
                                              color='grey'
                                              className='cursor-pointer'
                                              onClick={() =>
                                                updateCondition(
                                                  operation.id,
                                                  condition.id,
                                                  { path: pathItem },
                                                )
                                              }
                                            >
                                              {pathItem}
                                            </Tag>
                                          ),
                                        )}
                                      </Space>
                                    </Col>
                                    <Col xs={24} md={8}>
                                      <Text type='tertiary' size='small'>
                                        mode
                                      </Text>
                                      <Select
                                        value={condition.mode}
                                        optionList={CONDITION_MODE_OPTIONS}
                                        onChange={(nextValue) =>
                                          updateCondition(
                                            operation.id,
                                            condition.id,
                                            { mode: nextValue },
                                          )
                                        }
                                        style={{ width: '100%' }}
                                      />
                                    </Col>
                                    <Col xs={24} md={6}>
                                      <Text type='tertiary' size='small'>
                                        value
                                      </Text>
                                      <Input
                                        value={condition.value_text}
                                        placeholder='gpt'
                                        onChange={(nextValue) =>
                                          updateCondition(
                                            operation.id,
                                            condition.id,
                                            { value_text: nextValue },
                                          )
                                        }
                                      />
                                    </Col>
                                  </Row>
                                  <Space style={{ marginTop: 8 }}>
                                    <Switch
                                      checked={condition.invert}
                                      checkedText={t('开')}
                                      uncheckedText={t('关')}
                                      onChange={(nextValue) =>
                                        updateCondition(
                                          operation.id,
                                          condition.id,
                                          { invert: nextValue },
                                        )
                                      }
                                    />
                                    <Text type='tertiary' size='small'>
                                      invert
                                    </Text>
                                    <Switch
                                      checked={condition.pass_missing_key}
                                      checkedText={t('开')}
                                      uncheckedText={t('关')}
                                      onChange={(nextValue) =>
                                        updateCondition(
                                          operation.id,
                                          condition.id,
                                          { pass_missing_key: nextValue },
                                        )
                                      }
                                    />
                                    <Text type='tertiary' size='small'>
                                      pass_missing_key
                                    </Text>
                                  </Space>
                                </Card>
                              ))}
                            </Space>
                          )}
                        </div>
                      </Card>
                    );
                  })}
                </Space>
                <Card className='!rounded-xl border mt-3'>
                  <div className='flex items-center justify-between mb-2'>
                    <Text>{t('实时 JSON 预览')}</Text>
                    <Tag color='grey'>{t('预览')}</Tag>
                  </div>
                  <pre className='mb-0 text-xs leading-5 whitespace-pre-wrap break-all max-h-64 overflow-auto'>
                    {visualPreview || '{}'}
                  </pre>
                </Card>
              </div>
            )}
          </div>
        ) : (
          <div style={{ width: '100%' }}>
            <Space style={{ marginBottom: 8 }} wrap>
              <Button onClick={formatJson}>{t('格式化')}</Button>
              <Tag color='cyan'>{t('JSON 智能提示')}</Tag>
            </Space>
            <div className='border rounded-lg overflow-hidden'>
              <MonacoEditor
                beforeMount={configureMonaco}
                path={MONACO_MODEL_URI}
                language='json'
                value={jsonText}
                onChange={(nextValue) => handleJsonChange(nextValue ?? '')}
                height='460px'
                options={{
                  minimap: { enabled: false },
                  fontSize: 13,
                  lineNumbers: 'on',
                  automaticLayout: true,
                  scrollBeyondLastLine: false,
                  tabSize: 2,
                  insertSpaces: true,
                  wordWrap: 'on',
                  formatOnPaste: true,
                  formatOnType: true,
                }}
              />
            </div>
            <Text type='tertiary' size='small' className='mt-2 block'>
              {t('支持 mode/conditions 字段补全与 JSON Schema 校验')}
            </Text>
            {jsonError ? (
              <Text className='text-red-500 text-xs mt-2'>{jsonError}</Text>
            ) : null}
          </div>
        )}
      </Space>
    </Modal>
  );
};

export default ParamOverrideEditorModal;
