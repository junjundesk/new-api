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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Modal,
  Button,
  Typography,
  Checkbox,
  Input,
  Space,
} from '@douyinfe/semi-ui';
import { IconAlertTriangle } from '@douyinfe/semi-icons';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import MarkdownRenderer from '../markdown/MarkdownRenderer';

const { Text } = Typography;

const RiskAcknowledgementModal = ({
  visible,
  title,
  markdownContent = '',
  detailTitle = '',
  detailItems = [],
  checklist = [],
  inputPrompt = '',
  requiredText = '',
  inputPlaceholder = '',
  mismatchText = '',
  cancelText = '',
  confirmText = '',
  onCancel,
  onConfirm,
}) => {
  const isMobile = useIsMobile();
  const [checkedItems, setCheckedItems] = useState([]);
  const [typedText, setTypedText] = useState('');

  useEffect(() => {
    if (!visible) return;
    setCheckedItems(Array(checklist.length).fill(false));
    setTypedText('');
  }, [visible, checklist.length]);

  const allChecked = useMemo(() => {
    if (checklist.length === 0) return true;
    return checkedItems.length === checklist.length && checkedItems.every(Boolean);
  }, [checkedItems, checklist.length]);

  const typedMatched = useMemo(() => {
    if (!requiredText) return true;
    return typedText.trim() === requiredText.trim();
  }, [typedText, requiredText]);

  return (
    <Modal
      visible={visible}
      title={
        <Space align='center'>
          <IconAlertTriangle style={{ color: 'var(--semi-color-warning)' }} />
          <span>{title}</span>
        </Space>
      }
      width={isMobile ? '100%' : 860}
      centered
      maskClosable={false}
      closeOnEsc={false}
      onCancel={onCancel}
      bodyStyle={{
        maxHeight: isMobile ? '70vh' : '72vh',
        overflowY: 'auto',
        padding: isMobile ? '12px 16px' : '16px 20px',
      }}
      footer={
        <Space>
          <Button onClick={onCancel}>{cancelText}</Button>
          <Button
            theme='solid'
            type='danger'
            disabled={!allChecked || !typedMatched}
            onClick={onConfirm}
          >
            {confirmText}
          </Button>
        </Space>
      }
    >
      <div className='flex flex-col gap-4'>
        {markdownContent ? (
          <div className='border border-warning-200 bg-warning-50 rounded-md px-3 py-2'>
            <MarkdownRenderer content={markdownContent} />
          </div>
        ) : null}

        {detailItems.length > 0 ? (
          <div className='flex flex-col gap-2'>
            {detailTitle ? <Text strong>{detailTitle}</Text> : null}
            <div className='font-mono text-xs break-all bg-orange-50 border border-orange-200 rounded-md p-2'>
              {detailItems.join(', ')}
            </div>
          </div>
        ) : null}

        {checklist.length > 0 ? (
          <div className='flex flex-col gap-2'>
            {checklist.map((item, index) => (
              <Checkbox
                key={`risk-check-${index}`}
                checked={!!checkedItems[index]}
                onChange={(event) => {
                  const next = [...checkedItems];
                  next[index] = event.target.checked;
                  setCheckedItems(next);
                }}
              >
                {item}
              </Checkbox>
            ))}
          </div>
        ) : null}

        {requiredText ? (
          <div className='flex flex-col gap-2'>
            {inputPrompt ? <Text strong>{inputPrompt}</Text> : null}
            <div className='font-mono text-xs break-all bg-gray-50 border border-gray-200 rounded-md p-2'>
              {requiredText}
            </div>
            <Input
              value={typedText}
              onChange={setTypedText}
              placeholder={inputPlaceholder}
              onCopy={(event) => event.preventDefault()}
              onCut={(event) => event.preventDefault()}
              onPaste={(event) => event.preventDefault()}
              onDrop={(event) => event.preventDefault()}
            />
            {!typedMatched && typedText ? (
              <Text type='danger' size='small'>
                {mismatchText}
              </Text>
            ) : null}
          </div>
        ) : null}
      </div>
    </Modal>
  );
};

export default RiskAcknowledgementModal;
