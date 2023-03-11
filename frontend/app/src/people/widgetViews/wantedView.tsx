/* eslint-disable func-style */
import React, { useEffect, useState } from 'react';
import styled from 'styled-components';
import { useIsMobile } from '../../hooks';
import { extractGithubIssue, extractGithubIssueFromUrl } from '../../helpers';
import { useStores } from '../../store';
import PaidBounty from '../utils/paidBounty';
import Bounties from '../utils/assigned_unassigned_bounties';
import { colors } from '../../config/colors';
import MobileView from "./wantedViews/mobileView";
import DesktopView from "./wantedViews/desktopView";

export default function WantedView(props: any) {
  const {
    one_sentence_summary,
    title,
    description,
    priceMin,
    priceMax,
    price,
    person,
    created,
    issue,
    ticketUrl,
    repo,
    type,
    codingLanguage,
    assignee,
    estimate_session_length,
    loomEmbedUrl,
    onPanelClick
  } = props;
  const titleString = title ?? one_sentence_summary;

  let { show, paid } = props;
  const isMobile = useIsMobile();
  const { ui, main } = useStores();
  const [saving, setSaving] = useState(false);
  const [labels, setLabels] = useState([]);
  const { peopleWanteds } = main;
  const color = colors['light'];

  const isMine = ui.meInfo?.owner_pubkey === person?.owner_pubkey;

  if ('show' in props) {
    // show has a value
  } else {
    // if no value default to true
    show = true;
  }

  if ('paid' in props) {
    // show has no value
  } else {
    // if no value default to false
    paid = false;
  }

  async function setExtrasPropertyAndSave(propertyName: string) {
    if (peopleWanteds) {
      setSaving(true);
      try {
        const targetProperty = props[propertyName];
        const [clonedEx, targetIndex] = await main.setExtrasPropertyAndSave(
          'wanted',
          propertyName,
          created,
          !targetProperty
        );

        // saved? ok update in wanted list if found
        const peopleWantedsClone: any = [...peopleWanteds];
        const indexFromPeopleWanted = peopleWantedsClone.findIndex((f) => {
          const val = f.body || {};
          return f.person.owner_pubkey === ui.meInfo?.owner_pubkey && val.created === created;
        });

        // if we found it in the wanted list, update in people wanted list
        if (indexFromPeopleWanted > -1) {
          // if it should be hidden now, remove it from the list
          if ('show' in clonedEx[targetIndex] && clonedEx[targetIndex].show === false) {
            peopleWantedsClone.splice(indexFromPeopleWanted, 1);
          } else {
            peopleWantedsClone[indexFromPeopleWanted] = {
              person: person,
              body: clonedEx[targetIndex]
            };
          }
          main.setPeopleWanteds(peopleWantedsClone);
        }
      } catch (e) {
        console.log('e', e);
      }

      setSaving(false);
    }
  }

  useEffect(() => {
    if (codingLanguage) {
      const values = codingLanguage.map((value) => ({ ...value }));
      setLabels(values);
    }
  }, [codingLanguage]);

  function renderTickets() {
    const { status } = ticketUrl
      ? extractGithubIssueFromUrl(person, ticketUrl)
      : extractGithubIssue(person, repo, issue);

    const isClosed = status === 'closed' || paid ? true : false;

    const isCodingTask =
      type === 'coding_task' || type === 'wanted_coding_task' || type === 'freelance_job_request';

    // mobile view
    if (isMobile) {
      return (
        <MobileView
          {...props} labels={labels}
          key={ticketUrl}
          saving={saving}
          setExtrasPropertyAndSave={setExtrasPropertyAndSave}
          isClosed={isClosed}
          isCodingTask={isCodingTask}
          status={status}
          show={show}
          paid={paid}
          isMine={isMine}
          titleString={titleString}
        />
      )
    }

    if (props?.fromBountyPage) {
      return (
        <div>
          {paid ? (
            <BountyBox color={color}>
              <PaidBounty
                {...person}
                onPanelClick={onPanelClick}
                assignee={assignee}
                created={created}
                ticketUrl={ticketUrl}
                loomEmbedUrl={loomEmbedUrl}
                title={titleString}
                codingLanguage={labels}
                priceMin={priceMin}
                priceMax={priceMax}
                price={price}
                sessionLength={estimate_session_length}
                description={description}
              />
            </BountyBox>
          ) : (
            <BountyBox color={color}>
              <Bounties
                onPanelClick={onPanelClick}
                person={person}
                assignee={assignee}
                created={created}
                ticketUrl={ticketUrl}
                loomEmbedUrl={loomEmbedUrl}
                title={titleString}
                codingLanguage={labels}
                priceMin={priceMin}
                priceMax={priceMax}
                price={price}
                sessionLength={estimate_session_length}
                description={description}
              />
            </BountyBox>
          )}
        </div>
      );
    }

    return (
      <DesktopView
        {...props} labels={labels}
        saving={saving}
        setExtrasPropertyAndSave={setExtrasPropertyAndSave}
        isClosed={isClosed}
        isCodingTask={isCodingTask}
        status={status}
        show={show}
        paid={paid}
        isMine={isMine}
        titleString={titleString}
      />
    );
  }

  return renderTickets();
}

interface styledProps {
  color?: any;
}

const BountyBox = styled.div<styledProps>`
  min-height: 160px;
  max-height: 160px;
  width: 1100px;
  box-shadow: 0px 1px 6px ${(p) => p?.color && p?.color.black100};
  border: none;
`;