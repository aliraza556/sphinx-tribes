import moment from 'moment';
import { EuiDatePicker } from '@elastic/eui';
import React, { memo, useState } from 'react';
import { FieldEnv } from '..';
import styled from 'styled-components';
import { colors } from '../../../../config/colors';

function Date({ label, value, handleChange }: any) {
  const color = colors['light'];
  const [startDate, setStartDate] = useState(moment(value) ?? moment());
  const [isBorder, setIsBorder] = useState<boolean>(false);

  const handleChangeDate = (date) => {
    setStartDate(date);
    handleChange(date.toISOString());
  };

  return (
    <FieldEnv label={label} isTop={true} color={color}>
      <DataPicker
        selectsEnd={true}
        selectsStart={true}
        selected={startDate}
        onChange={(e) => {
          handleChangeDate(e);
        }}
        onFocus={() => {
          setIsBorder(true);
        }}
        border={isBorder}
        color={color}
      />
    </FieldEnv>
  );
}
export default memo(Date);

interface datePickerProps {
  border?: boolean;
  color?: any;
}

// @ts-ignore
const DataPicker = styled(EuiDatePicker) <datePickerProps>`
  border: 1px solid ${(p) => (p.border ? p?.color?.blue2 : p?.color?.grayish.G600)};
  :focus {
    background-image: none;
  }
`;