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

import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Card,
  Col,
  Collapse,
  Input,
  Modal,
  Row,
  Select,
  Space,
  Switch,
  Tag,
  TextArea,
  Typography,
} from '@douyinfe/semi-ui';
import { IconDelete, IconPlus } from '@douyinfe/semi-icons';
import { showError, verifyJSON } from '../../../../helpers';

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
  { label: 'Control · sync_fields', value: 'sync_fields' },
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
  sync_fields: { from: true, to: true },
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
  'sync_fields',
]);

const TO_REQUIRED_MODES = new Set([
  'copy',
  'move',
  'copy_header',
  'move_header',
  'sync_fields',
]);

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
  sync_fields: 'Sync two fields when one exists and the other is missing',
  set_header: 'Set runtime override header',
  delete_header: 'Delete runtime override header',
  copy_header: 'Copy header from from -> to',
  move_header: 'Move header from from -> to',
};

const SYNC_TARGET_TYPE_OPTIONS = [
  { label: 'JSON', value: 'json' },
  { label: 'Header', value: 'header' },
];

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

const TEMPLATE_LIBRARY_OPTIONS = [
  { label: 'Template · Operations', value: 'operations' },
  { label: 'Template · Legacy Object', value: 'legacy' },
];

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

const parseSyncTargetSpec = (spec) => {
  const raw = String(spec ?? '').trim();
  if (!raw) return { type: 'json', key: '' };
  const idx = raw.indexOf(':');
  if (idx < 0) return { type: 'json', key: raw };
  const prefix = raw.slice(0, idx).trim().toLowerCase();
  const key = raw.slice(idx + 1).trim();
  if (prefix === 'header') {
    return { type: 'header', key };
  }
  return { type: 'json', key };
};

const buildSyncTargetSpec = (type, key) => {
  const normalizedType = type === 'header' ? 'header' : 'json';
  const normalizedKey = String(key ?? '').trim();
  if (!normalizedKey) return '';
  return `${normalizedType}:${normalizedKey}`;
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

const getOperationSummary = (operation = {}, index = 0) => {
  const mode = operation.mode || 'set';
  if (mode === 'sync_fields') {
    const from = String(operation.from || '').trim();
    const to = String(operation.to || '').trim();
    return `${index + 1}. ${mode} · ${from || to || '-'}`;
  }
  const path = String(operation.path || '').trim();
  const from = String(operation.from || '').trim();
  const to = String(operation.to || '').trim();
  return `${index + 1}. ${mode} · ${path || from || to || '-'}`;
};

const getOperationModeTagColor = (mode = 'set') => {
  if (mode.includes('header')) return 'cyan';
  if (mode.includes('replace') || mode.includes('trim')) return 'violet';
  if (mode.includes('copy') || mode.includes('move')) return 'blue';
  if (mode.includes('error') || mode.includes('prune')) return 'red';
  if (mode.includes('sync')) return 'green';
  return 'grey';
};

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
  const [operationSearch, setOperationSearch] = useState('');
  const [selectedOperationId, setSelectedOperationId] = useState('');
  const [expandedConditionMap, setExpandedConditionMap] = useState({});
  const [templateLibraryKey, setTemplateLibraryKey] = useState('operations');

  useEffect(() => {
    if (!visible) return;
    const nextState = parseInitialState(value);
    setEditMode(nextState.editMode);
    setVisualMode(nextState.visualMode);
    setLegacyValue(nextState.legacyValue);
    setOperations(nextState.operations);
    setJsonText(nextState.jsonText);
    setJsonError(nextState.jsonError);
    setOperationSearch('');
    setSelectedOperationId(nextState.operations[0]?.id || '');
    setExpandedConditionMap({});
    setTemplateLibraryKey(
      nextState.visualMode === 'legacy' ? 'legacy' : 'operations',
    );
  }, [visible, value]);

  useEffect(() => {
    if (operations.length === 0) {
      setSelectedOperationId('');
      return;
    }
    if (!operations.some((item) => item.id === selectedOperationId)) {
      setSelectedOperationId(operations[0].id);
    }
  }, [operations, selectedOperationId]);

  useEffect(() => {
    setTemplateLibraryKey(visualMode === 'legacy' ? 'legacy' : 'operations');
  }, [visualMode]);

  const operationCount = useMemo(
    () => operations.filter((item) => !isOperationBlank(item)).length,
    [operations],
  );

  const filteredOperations = useMemo(() => {
    const keyword = operationSearch.trim().toLowerCase();
    if (!keyword) return operations;
    return operations.filter((operation) => {
      const searchableText = [
        operation.mode,
        operation.path,
        operation.from,
        operation.to,
        operation.value_text,
      ]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return searchableText.includes(keyword);
    });
  }, [operationSearch, operations]);

  const selectedOperation = useMemo(
    () => operations.find((operation) => operation.id === selectedOperationId),
    [operations, selectedOperationId],
  );

  const selectedOperationIndex = useMemo(
    () =>
      operations.findIndex((operation) => operation.id === selectedOperationId),
    [operations, selectedOperationId],
  );

  const topOperationModes = useMemo(() => {
    const counts = operations.reduce((acc, operation) => {
      const mode = operation.mode || 'set';
      acc[mode] = (acc[mode] || 0) + 1;
      return acc;
    }, {});
    return Object.entries(counts)
      .sort((a, b) => b[1] - a[1])
      .slice(0, 4);
  }, [operations]);

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
      const fallback = createDefaultOperation();
      setVisualMode('operations');
      setOperations([fallback]);
      setSelectedOperationId(fallback.id);
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
      const nextOperations =
        parsed.operations.length > 0
          ? parsed.operations.map(normalizeOperation)
          : [createDefaultOperation()];
      setVisualMode('operations');
      setOperations(nextOperations);
      setSelectedOperationId(nextOperations[0]?.id || '');
      setLegacyValue('');
      setJsonError('');
      setEditMode('visual');
      return;
    }
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      const fallback = createDefaultOperation();
      setVisualMode('legacy');
      setLegacyValue(JSON.stringify(parsed, null, 2));
      setOperations([fallback]);
      setSelectedOperationId(fallback.id);
      setJsonError('');
      setEditMode('visual');
      return;
    }
    showError(t('参数覆盖必须是合法的 JSON 对象'));
  };

  const setOldTemplate = () => {
    const text = JSON.stringify(LEGACY_TEMPLATE, null, 2);
    const fallback = createDefaultOperation();
    setVisualMode('legacy');
    setLegacyValue(text);
    setOperations([fallback]);
    setSelectedOperationId(fallback.id);
    setExpandedConditionMap({});
    setJsonText(text);
    setJsonError('');
    setEditMode('visual');
    setTemplateLibraryKey('legacy');
  };

  const setNewTemplate = () => {
    const nextOperations =
      OPERATION_TEMPLATE.operations.map(normalizeOperation);
    setVisualMode('operations');
    setOperations(nextOperations);
    setSelectedOperationId(nextOperations[0]?.id || '');
    setExpandedConditionMap({});
    setJsonText(JSON.stringify(OPERATION_TEMPLATE, null, 2));
    setJsonError('');
    setEditMode('visual');
    setTemplateLibraryKey('operations');
  };

  const clearValue = () => {
    const fallback = createDefaultOperation();
    setVisualMode('operations');
    setLegacyValue('');
    setOperations([fallback]);
    setSelectedOperationId(fallback.id);
    setExpandedConditionMap({});
    setJsonText('');
    setJsonError('');
    setTemplateLibraryKey('operations');
  };

  const applyTemplateFromLibrary = () => {
    if (templateLibraryKey === 'legacy') {
      setOldTemplate();
      return;
    }
    setNewTemplate();
  };

  const resetEditorState = () => {
    clearValue();
    setEditMode('visual');
  };

  const updateOperation = (operationId, patch) => {
    setOperations((prev) =>
      prev.map((item) =>
        item.id === operationId ? { ...item, ...patch } : item,
      ),
    );
  };

  const addOperation = () => {
    const created = createDefaultOperation();
    setOperations((prev) => [...prev, created]);
    setSelectedOperationId(created.id);
  };

  const duplicateOperation = (operationId) => {
    let insertedId = '';
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
      insertedId = cloned.id;
      const next = [...prev];
      next.splice(index + 1, 0, cloned);
      return next;
    });
    if (insertedId) {
      setSelectedOperationId(insertedId);
    }
  };

  const removeOperation = (operationId) => {
    setOperations((prev) => {
      if (prev.length <= 1) return [createDefaultOperation()];
      return prev.filter((item) => item.id !== operationId);
    });
    setExpandedConditionMap((prev) => {
      if (!Object.prototype.hasOwnProperty.call(prev, operationId)) {
        return prev;
      }
      const next = { ...prev };
      delete next[operationId];
      return next;
    });
  };

  const addCondition = (operationId) => {
    const createdCondition = createDefaultCondition();
    setOperations((prev) =>
      prev.map((operation) =>
        operation.id === operationId
          ? {
              ...operation,
              conditions: [...(operation.conditions || []), createdCondition],
            }
          : operation,
      ),
    );
    setExpandedConditionMap((prev) => ({
      ...prev,
      [operationId]: [...(prev[operationId] || []), createdCondition.id],
    }));
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
    setExpandedConditionMap((prev) => ({
      ...prev,
      [operationId]: (prev[operationId] || []).filter(
        (id) => id !== conditionId,
      ),
    }));
  };

  const selectedConditionKeys = useMemo(
    () => expandedConditionMap[selectedOperationId] || [],
    [expandedConditionMap, selectedOperationId],
  );

  const handleConditionCollapseChange = useCallback(
    (operationId, activeKeys) => {
      const keys = (
        Array.isArray(activeKeys) ? activeKeys : [activeKeys]
      ).filter(Boolean);
      setExpandedConditionMap((prev) => ({
        ...prev,
        [operationId]: keys,
      }));
    },
    [],
  );

  const expandAllSelectedConditions = useCallback(() => {
    if (!selectedOperationId || !selectedOperation) return;
    setExpandedConditionMap((prev) => ({
      ...prev,
      [selectedOperationId]: (selectedOperation.conditions || []).map(
        (condition) => condition.id,
      ),
    }));
  }, [selectedOperation, selectedOperationId]);

  const collapseAllSelectedConditions = useCallback(() => {
    if (!selectedOperationId) return;
    setExpandedConditionMap((prev) => ({
      ...prev,
      [selectedOperationId]: [],
    }));
  }, [selectedOperationId]);

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
      width={1120}
      onCancel={onCancel}
      onOk={handleSave}
      okText={t('保存')}
      cancelText={t('取消')}
    >
      <Space vertical align='start' spacing={12} style={{ width: '100%' }}>
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
          <Select
            value={templateLibraryKey}
            optionList={TEMPLATE_LIBRARY_OPTIONS}
            onChange={(nextValue) =>
              setTemplateLibraryKey(nextValue || 'operations')
            }
            style={{ width: 240 }}
          />
          <Button onClick={applyTemplateFromLibrary}>{t('应用模板')}</Button>
          <Button onClick={resetEditorState}>{t('重置')}</Button>
        </Space>

        {editMode === 'visual' ? (
          <div style={{ width: '100%' }}>
            {visualMode === 'legacy' ? (
              <div>
                <Text className='mb-2 block'>{t('旧格式（直接覆盖）：')}</Text>
                <TextArea
                  value={legacyValue}
                  autosize={{ minRows: 10, maxRows: 20 }}
                  placeholder={JSON.stringify(LEGACY_TEMPLATE, null, 2)}
                  onChange={(nextValue) => setLegacyValue(nextValue)}
                  showClear
                />
                <Text type='tertiary' size='small' className='mt-2 block'>
                  {t('这里直接编辑 JSON 对象，无需额外点开编辑器。')}
                </Text>
              </div>
            ) : (
              <div>
                <div className='flex items-center justify-between mb-3'>
                  <Space>
                    <Text>{t('新格式（支持条件判断与json自定义）：')}</Text>
                    <Tag color='cyan'>{`${t('规则')}: ${operationCount}`}</Tag>
                  </Space>
                  <Button icon={<IconPlus />} onClick={addOperation}>
                    {t('新增规则')}
                  </Button>
                </div>

                <Row gutter={12}>
                  <Col xs={24} md={8}>
                    <Card
                      className='!rounded-2xl !border-0 h-full'
                      bodyStyle={{
                        padding: 12,
                        background: 'var(--semi-color-fill-0)',
                        display: 'flex',
                        flexDirection: 'column',
                        gap: 10,
                        minHeight: 520,
                      }}
                    >
                      <div className='flex items-center justify-between'>
                        <Text strong>{t('规则导航')}</Text>
                        <Tag color='grey'>{`${operationCount}/${operations.length}`}</Tag>
                      </div>

                      {topOperationModes.length > 0 ? (
                        <Space wrap spacing={6}>
                          {topOperationModes.map(([mode, count]) => (
                            <Tag
                              key={`mode_stat_${mode}`}
                              size='small'
                              color={getOperationModeTagColor(mode)}
                            >
                              {`${mode} · ${count}`}
                            </Tag>
                          ))}
                        </Space>
                      ) : null}

                      <Input
                        value={operationSearch}
                        placeholder={t('搜索规则（mode/path/from/to）')}
                        onChange={(nextValue) =>
                          setOperationSearch(nextValue || '')
                        }
                        showClear
                      />

                      <div
                        className='overflow-auto'
                        style={{ flex: 1, minHeight: 320, paddingRight: 2 }}
                      >
                        {filteredOperations.length === 0 ? (
                          <Text type='tertiary' size='small'>
                            {t('没有匹配的规则')}
                          </Text>
                        ) : (
                          <div
                            style={{
                              display: 'flex',
                              flexDirection: 'column',
                              gap: 8,
                              width: '100%',
                            }}
                          >
                            {filteredOperations.map((operation) => {
                              const index = operations.findIndex(
                                (item) => item.id === operation.id,
                              );
                              const isActive =
                                operation.id === selectedOperationId;
                              return (
                                <div
                                  key={operation.id}
                                  role='button'
                                  tabIndex={0}
                                  onClick={() =>
                                    setSelectedOperationId(operation.id)
                                  }
                                  onKeyDown={(event) => {
                                    if (
                                      event.key === 'Enter' ||
                                      event.key === ' '
                                    ) {
                                      event.preventDefault();
                                      setSelectedOperationId(operation.id);
                                    }
                                  }}
                                  className='w-full rounded-xl px-3 py-3 cursor-pointer transition-colors'
                                  style={{
                                    background: isActive
                                      ? 'var(--semi-color-primary-light-default)'
                                      : 'var(--semi-color-bg-2)',
                                    border: isActive
                                      ? '1px solid var(--semi-color-primary)'
                                      : '1px solid var(--semi-color-border)',
                                  }}
                                >
                                  <div className='flex items-start justify-between gap-2'>
                                    <div>
                                      <Text strong>{`#${index + 1}`}</Text>
                                      <Text
                                        type='tertiary'
                                        size='small'
                                        className='block mt-1'
                                      >
                                        {getOperationSummary(operation, index)}
                                      </Text>
                                    </div>
                                    <Tag size='small' color='grey'>
                                      {(operation.conditions || []).length}
                                    </Tag>
                                  </div>
                                  <Space spacing={6} style={{ marginTop: 8 }}>
                                    <Tag
                                      size='small'
                                      color={getOperationModeTagColor(
                                        operation.mode || 'set',
                                      )}
                                    >
                                      {operation.mode || 'set'}
                                    </Tag>
                                    <Text type='tertiary' size='small'>
                                      {t('条件')}
                                    </Text>
                                  </Space>
                                </div>
                              );
                            })}
                          </div>
                        )}
                      </div>
                    </Card>
                  </Col>
                  <Col xs={24} md={16}>
                    {selectedOperation ? (
                      (() => {
                        const mode = selectedOperation.mode || 'set';
                        const meta = MODE_META[mode] || MODE_META.set;
                        const conditions = selectedOperation.conditions || [];
                        const syncFromTarget =
                          mode === 'sync_fields'
                            ? parseSyncTargetSpec(selectedOperation.from)
                            : null;
                        const syncToTarget =
                          mode === 'sync_fields'
                            ? parseSyncTargetSpec(selectedOperation.to)
                            : null;
                        return (
                          <Card
                            className='!rounded-2xl !border-0'
                            bodyStyle={{
                              padding: 14,
                              background: 'var(--semi-color-fill-0)',
                            }}
                          >
                            <div className='flex items-center justify-between mb-3'>
                              <Space>
                                <Tag color='blue'>{`#${selectedOperationIndex + 1}`}</Tag>
                                <Text strong>
                                  {getOperationSummary(
                                    selectedOperation,
                                    selectedOperationIndex,
                                  )}
                                </Text>
                              </Space>
                              <Space>
                                <Button
                                  size='small'
                                  type='tertiary'
                                  onClick={() =>
                                    duplicateOperation(selectedOperation.id)
                                  }
                                >
                                  {t('复制')}
                                </Button>
                                <Button
                                  size='small'
                                  type='danger'
                                  theme='borderless'
                                  icon={<IconDelete />}
                                  onClick={() =>
                                    removeOperation(selectedOperation.id)
                                  }
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
                                    updateOperation(selectedOperation.id, {
                                      mode: nextMode,
                                    })
                                  }
                                  style={{ width: '100%' }}
                                />
                              </Col>
                              {meta.path || meta.pathOptional ? (
                                <Col xs={24} md={16}>
                                  <Text type='tertiary' size='small'>
                                    {meta.pathOptional
                                      ? 'path (optional)'
                                      : 'path'}
                                  </Text>
                                  <Input
                                    value={selectedOperation.path}
                                    placeholder={
                                      mode.includes('header')
                                        ? 'X-Debug-Mode'
                                        : mode === 'prune_objects'
                                          ? 'messages (optional)'
                                          : 'temperature'
                                    }
                                    onChange={(nextValue) =>
                                      updateOperation(selectedOperation.id, {
                                        path: nextValue,
                                      })
                                    }
                                  />
                                  <Space wrap style={{ marginTop: 6 }}>
                                    {OPERATION_PATH_SUGGESTIONS.map(
                                      (pathItem) => (
                                        <Tag
                                          key={`${selectedOperation.id}_${pathItem}`}
                                          size='small'
                                          color='grey'
                                          className='cursor-pointer'
                                          onClick={() =>
                                            updateOperation(
                                              selectedOperation.id,
                                              {
                                                path: pathItem,
                                              },
                                            )
                                          }
                                        >
                                          {pathItem}
                                        </Tag>
                                      ),
                                    )}
                                  </Space>
                                </Col>
                              ) : null}
                            </Row>

                            <Text
                              type='tertiary'
                              size='small'
                              className='mt-1 block'
                            >
                              {MODE_DESCRIPTIONS[mode] || ''}
                            </Text>

                            {meta.value ? (
                              <div className='mt-2'>
                                <Text type='tertiary' size='small'>
                                  value (JSON or plain text)
                                </Text>
                                <TextArea
                                  value={selectedOperation.value_text}
                                  autosize={{ minRows: 1, maxRows: 4 }}
                                  placeholder='0.7'
                                  onChange={(nextValue) =>
                                    updateOperation(selectedOperation.id, {
                                      value_text: nextValue,
                                    })
                                  }
                                />
                              </div>
                            ) : null}

                            {meta.keepOrigin ? (
                              <div className='mt-2'>
                                <Switch
                                  checked={Boolean(
                                    selectedOperation.keep_origin,
                                  )}
                                  checkedText={t('开')}
                                  uncheckedText={t('关')}
                                  onChange={(nextValue) =>
                                    updateOperation(selectedOperation.id, {
                                      keep_origin: nextValue,
                                    })
                                  }
                                />
                                <Text
                                  type='tertiary'
                                  size='small'
                                  className='ml-2'
                                >
                                  keep_origin
                                </Text>
                              </div>
                            ) : null}

                            {mode === 'sync_fields' ? (
                              <div className='mt-2'>
                                <Text type='tertiary' size='small'>
                                  sync endpoints
                                </Text>
                                <Row gutter={12} style={{ marginTop: 6 }}>
                                  <Col xs={24} md={12}>
                                    <Text type='tertiary' size='small'>
                                      from endpoint
                                    </Text>
                                    <div className='flex gap-2'>
                                      <Select
                                        value={syncFromTarget?.type || 'json'}
                                        optionList={SYNC_TARGET_TYPE_OPTIONS}
                                        style={{ width: 120 }}
                                        onChange={(nextType) =>
                                          updateOperation(
                                            selectedOperation.id,
                                            {
                                              from: buildSyncTargetSpec(
                                                nextType,
                                                syncFromTarget?.key || '',
                                              ),
                                            },
                                          )
                                        }
                                      />
                                      <Input
                                        value={syncFromTarget?.key || ''}
                                        placeholder='session_id'
                                        onChange={(nextKey) =>
                                          updateOperation(
                                            selectedOperation.id,
                                            {
                                              from: buildSyncTargetSpec(
                                                syncFromTarget?.type || 'json',
                                                nextKey,
                                              ),
                                            },
                                          )
                                        }
                                      />
                                    </div>
                                  </Col>
                                  <Col xs={24} md={12}>
                                    <Text type='tertiary' size='small'>
                                      to endpoint
                                    </Text>
                                    <div className='flex gap-2'>
                                      <Select
                                        value={syncToTarget?.type || 'json'}
                                        optionList={SYNC_TARGET_TYPE_OPTIONS}
                                        style={{ width: 120 }}
                                        onChange={(nextType) =>
                                          updateOperation(
                                            selectedOperation.id,
                                            {
                                              to: buildSyncTargetSpec(
                                                nextType,
                                                syncToTarget?.key || '',
                                              ),
                                            },
                                          )
                                        }
                                      />
                                      <Input
                                        value={syncToTarget?.key || ''}
                                        placeholder='prompt_cache_key'
                                        onChange={(nextKey) =>
                                          updateOperation(
                                            selectedOperation.id,
                                            {
                                              to: buildSyncTargetSpec(
                                                syncToTarget?.type || 'json',
                                                nextKey,
                                              ),
                                            },
                                          )
                                        }
                                      />
                                    </div>
                                  </Col>
                                </Row>
                                <Space wrap style={{ marginTop: 8 }}>
                                  <Tag
                                    size='small'
                                    color='cyan'
                                    className='cursor-pointer'
                                    onClick={() =>
                                      updateOperation(selectedOperation.id, {
                                        from: 'header:session_id',
                                        to: 'json:prompt_cache_key',
                                      })
                                    }
                                  >
                                    {
                                      'header:session_id -> json:prompt_cache_key'
                                    }
                                  </Tag>
                                  <Tag
                                    size='small'
                                    color='cyan'
                                    className='cursor-pointer'
                                    onClick={() =>
                                      updateOperation(selectedOperation.id, {
                                        from: 'json:prompt_cache_key',
                                        to: 'header:session_id',
                                      })
                                    }
                                  >
                                    {
                                      'json:prompt_cache_key -> header:session_id'
                                    }
                                  </Tag>
                                </Space>
                              </div>
                            ) : meta.from || meta.to === false || meta.to ? (
                              <Row gutter={12} style={{ marginTop: 8 }}>
                                {meta.from || meta.to === false ? (
                                  <Col xs={24} md={12}>
                                    <Text type='tertiary' size='small'>
                                      from
                                    </Text>
                                    <Input
                                      value={selectedOperation.from}
                                      placeholder={
                                        mode.includes('header')
                                          ? 'Authorization'
                                          : 'model'
                                      }
                                      onChange={(nextValue) =>
                                        updateOperation(selectedOperation.id, {
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
                                      value={selectedOperation.to}
                                      placeholder={
                                        mode.includes('header')
                                          ? 'X-Upstream-Auth'
                                          : 'original_model'
                                      }
                                      onChange={(nextValue) =>
                                        updateOperation(selectedOperation.id, {
                                          to: nextValue,
                                        })
                                      }
                                    />
                                  </Col>
                                ) : null}
                              </Row>
                            ) : null}

                            <div
                              className='mt-3 rounded-xl p-2'
                              style={{
                                background: 'rgba(127, 127, 127, 0.08)',
                              }}
                            >
                              <div className='flex items-center justify-between mb-2'>
                                <Space>
                                  <Text>{t('条件')}</Text>
                                  <Select
                                    value={selectedOperation.logic || 'OR'}
                                    optionList={[
                                      { label: 'OR', value: 'OR' },
                                      { label: 'AND', value: 'AND' },
                                    ]}
                                    style={{ width: 96 }}
                                    onChange={(nextValue) =>
                                      updateOperation(selectedOperation.id, {
                                        logic: nextValue,
                                      })
                                    }
                                  />
                                </Space>
                                <Space spacing={6}>
                                  <Button
                                    size='small'
                                    type='tertiary'
                                    onClick={expandAllSelectedConditions}
                                  >
                                    {t('全部展开')}
                                  </Button>
                                  <Button
                                    size='small'
                                    type='tertiary'
                                    onClick={collapseAllSelectedConditions}
                                  >
                                    {t('全部收起')}
                                  </Button>
                                  <Button
                                    icon={<IconPlus />}
                                    size='small'
                                    onClick={() =>
                                      addCondition(selectedOperation.id)
                                    }
                                  >
                                    {t('新增条件')}
                                  </Button>
                                </Space>
                              </div>

                              {conditions.length === 0 ? (
                                <Text type='tertiary' size='small'>
                                  {t('没有条件时，默认总是执行该操作。')}
                                </Text>
                              ) : (
                                <Collapse
                                  keepDOM
                                  activeKey={selectedConditionKeys}
                                  onChange={(activeKeys) =>
                                    handleConditionCollapseChange(
                                      selectedOperation.id,
                                      activeKeys,
                                    )
                                  }
                                >
                                  {conditions.map(
                                    (condition, conditionIndex) => (
                                      <Collapse.Panel
                                        key={condition.id}
                                        itemKey={condition.id}
                                        header={
                                          <Space spacing={8}>
                                            <Tag size='small'>
                                              {`C${conditionIndex + 1}`}
                                            </Tag>
                                            <Text type='tertiary' size='small'>
                                              {condition.path ||
                                                t('未设置 path')}
                                            </Text>
                                          </Space>
                                        }
                                      >
                                        <div>
                                          <div className='flex justify-end mb-2'>
                                            <Button
                                              theme='borderless'
                                              type='danger'
                                              icon={<IconDelete />}
                                              size='small'
                                              onClick={() =>
                                                removeCondition(
                                                  selectedOperation.id,
                                                  condition.id,
                                                )
                                              }
                                            />
                                          </div>
                                          <Row gutter={12}>
                                            <Col xs={24} md={10}>
                                              <Text
                                                type='tertiary'
                                                size='small'
                                              >
                                                path
                                              </Text>
                                              <Input
                                                value={condition.path}
                                                placeholder='model'
                                                onChange={(nextValue) =>
                                                  updateCondition(
                                                    selectedOperation.id,
                                                    condition.id,
                                                    { path: nextValue },
                                                  )
                                                }
                                              />
                                              <Space
                                                wrap
                                                style={{ marginTop: 6 }}
                                              >
                                                {CONDITION_PATH_SUGGESTIONS.map(
                                                  (pathItem) => (
                                                    <Tag
                                                      key={`${condition.id}_${pathItem}`}
                                                      size='small'
                                                      color='grey'
                                                      className='cursor-pointer'
                                                      onClick={() =>
                                                        updateCondition(
                                                          selectedOperation.id,
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
                                              <Text
                                                type='tertiary'
                                                size='small'
                                              >
                                                mode
                                              </Text>
                                              <Select
                                                value={condition.mode}
                                                optionList={
                                                  CONDITION_MODE_OPTIONS
                                                }
                                                onChange={(nextValue) =>
                                                  updateCondition(
                                                    selectedOperation.id,
                                                    condition.id,
                                                    { mode: nextValue },
                                                  )
                                                }
                                                style={{ width: '100%' }}
                                              />
                                            </Col>
                                            <Col xs={24} md={6}>
                                              <Text
                                                type='tertiary'
                                                size='small'
                                              >
                                                value
                                              </Text>
                                              <Input
                                                value={condition.value_text}
                                                placeholder='gpt'
                                                onChange={(nextValue) =>
                                                  updateCondition(
                                                    selectedOperation.id,
                                                    condition.id,
                                                    { value_text: nextValue },
                                                  )
                                                }
                                              />
                                            </Col>
                                          </Row>
                                          <Space style={{ marginTop: 8 }}>
                                            <Switch
                                              checked={Boolean(
                                                condition.invert,
                                              )}
                                              checkedText={t('开')}
                                              uncheckedText={t('关')}
                                              onChange={(nextValue) =>
                                                updateCondition(
                                                  selectedOperation.id,
                                                  condition.id,
                                                  { invert: nextValue },
                                                )
                                              }
                                            />
                                            <Text type='tertiary' size='small'>
                                              invert
                                            </Text>
                                            <Switch
                                              checked={Boolean(
                                                condition.pass_missing_key,
                                              )}
                                              checkedText={t('开')}
                                              uncheckedText={t('关')}
                                              onChange={(nextValue) =>
                                                updateCondition(
                                                  selectedOperation.id,
                                                  condition.id,
                                                  {
                                                    pass_missing_key: nextValue,
                                                  },
                                                )
                                              }
                                            />
                                            <Text type='tertiary' size='small'>
                                              pass_missing_key
                                            </Text>
                                          </Space>
                                        </div>
                                      </Collapse.Panel>
                                    ),
                                  )}
                                </Collapse>
                              )}
                            </div>
                          </Card>
                        );
                      })()
                    ) : (
                      <Card
                        className='!rounded-2xl !border-0'
                        bodyStyle={{
                          padding: 14,
                          background: 'var(--semi-color-fill-0)',
                        }}
                      >
                        <Text type='tertiary'>
                          {t('请选择一条规则进行编辑。')}
                        </Text>
                      </Card>
                    )}

                    <Card
                      className='!rounded-2xl !border-0 mt-3'
                      bodyStyle={{
                        padding: 12,
                        background: 'var(--semi-color-fill-0)',
                      }}
                    >
                      <div className='flex items-center justify-between mb-2'>
                        <Text>{t('实时 JSON 预览')}</Text>
                        <Tag color='grey'>{t('预览')}</Tag>
                      </div>
                      <pre className='mb-0 text-xs leading-5 whitespace-pre-wrap break-all max-h-64 overflow-auto'>
                        {visualPreview || '{}'}
                      </pre>
                    </Card>
                  </Col>
                </Row>
              </div>
            )}
          </div>
        ) : (
          <div style={{ width: '100%' }}>
            <Space style={{ marginBottom: 8 }} wrap>
              <Button onClick={formatJson}>{t('格式化')}</Button>
              <Tag color='grey'>{t('普通编辑')}</Tag>
            </Space>
            <TextArea
              value={jsonText}
              autosize={{ minRows: 18, maxRows: 28 }}
              onChange={(nextValue) => handleJsonChange(nextValue ?? '')}
              placeholder={JSON.stringify(OPERATION_TEMPLATE, null, 2)}
              showClear
            />
            <Text type='tertiary' size='small' className='mt-2 block'>
              {t('直接编辑 JSON 文本，保存时会校验格式。')}
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
