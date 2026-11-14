import React from 'react';
import { Select } from 'antd';
import type { SelectProps } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { listEpics } from '../../api/epic';

const CLOSED = ['done', 'closed', 'rejected', 'duplicate'];

/**
 * Picks one of a project's epics by key (MF-39). Closed epics are left out
 * unless includeClosed — new work should not land in a finished epic. Works
 * as a Form.Item control (value/onChange carry the epic key).
 */
const EpicSelect: React.FC<{ projectId: string; includeClosed?: boolean } & Omit<SelectProps<string>, 'options'>> = ({
  projectId,
  includeClosed,
  ...rest
}) => {
  const { data: epics = [] } = useQuery({
    queryKey: ['epics', projectId],
    queryFn: () => listEpics(projectId),
    enabled: !!projectId,
  });
  const options = epics
    .filter((e) => includeClosed || !CLOSED.includes(e.status))
    .map((e) => ({ value: e.issue_key, label: `${e.issue_key} ${e.title}` }));
  return <Select allowClear showSearch optionFilterProp="label" placeholder="Epic" options={options} {...rest} />;
};

export default EpicSelect;
