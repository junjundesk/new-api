import React from 'react';
import { useTranslation } from 'react-i18next';
import RiskAcknowledgementModal from '../../../common/modals/RiskAcknowledgementModal';
import {
  STATUS_CODE_RISK_I18N_KEYS,
  STATUS_CODE_RISK_CHECKLIST_KEYS,
} from './statusCodeRiskGuard';

const StatusCodeRiskGuardModal = ({
  visible,
  detailItems,
  onCancel,
  onConfirm,
}) => {
  const { t } = useTranslation();

  return (
    <RiskAcknowledgementModal
      visible={visible}
      title={t(STATUS_CODE_RISK_I18N_KEYS.title)}
      markdownContent={t(STATUS_CODE_RISK_I18N_KEYS.markdown)}
      detailTitle={t(STATUS_CODE_RISK_I18N_KEYS.detailTitle)}
      detailItems={detailItems}
      checklist={STATUS_CODE_RISK_CHECKLIST_KEYS.map((item) => t(item))}
      inputPrompt={t(STATUS_CODE_RISK_I18N_KEYS.inputPrompt)}
      requiredText={t(STATUS_CODE_RISK_I18N_KEYS.confirmText)}
      inputPlaceholder={t(STATUS_CODE_RISK_I18N_KEYS.inputPlaceholder)}
      mismatchText={t(STATUS_CODE_RISK_I18N_KEYS.mismatchText)}
      cancelText={t('取消')}
      confirmText={t(STATUS_CODE_RISK_I18N_KEYS.confirmButton)}
      onCancel={onCancel}
      onConfirm={onConfirm}
    />
  );
};

export default StatusCodeRiskGuardModal;
