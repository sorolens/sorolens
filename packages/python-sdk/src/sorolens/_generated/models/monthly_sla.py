from __future__ import annotations

import datetime
from collections.abc import Mapping
from typing import Any, TypeVar, cast

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

T = TypeVar("T", bound="MonthlySLA")


@_attrs_define
class MonthlySLA:
    """
    Attributes:
        contract_id (str):
        month (str): Reporting period, YYYY-MM (UTC).
        uptime_pct (float): healthy_checks / total_checks * 100; 0 when there were no checks.
        total_checks (int):
        healthy_checks (int):
        incidents (int): Outages (a transition from Healthy into any other status) in the month.
        mttr_seconds (float): Mean time to recovery across incidents that recovered within the month.
        total_downtime_seconds (float):
        longest_outage_seconds (float):
        ongoing_outage (bool): True when the month ends mid-incident (MTTR then understates reality).
        critical_alerts (int):
        warning_alerts (int):
        info_alerts (int):
        total_alerts (int):
        first_check (datetime.datetime | None):
        last_check (datetime.datetime | None):
    """

    contract_id: str
    month: str
    uptime_pct: float
    total_checks: int
    healthy_checks: int
    incidents: int
    mttr_seconds: float
    total_downtime_seconds: float
    longest_outage_seconds: float
    ongoing_outage: bool
    critical_alerts: int
    warning_alerts: int
    info_alerts: int
    total_alerts: int
    first_check: datetime.datetime | None
    last_check: datetime.datetime | None
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        contract_id = self.contract_id

        month = self.month

        uptime_pct = self.uptime_pct

        total_checks = self.total_checks

        healthy_checks = self.healthy_checks

        incidents = self.incidents

        mttr_seconds = self.mttr_seconds

        total_downtime_seconds = self.total_downtime_seconds

        longest_outage_seconds = self.longest_outage_seconds

        ongoing_outage = self.ongoing_outage

        critical_alerts = self.critical_alerts

        warning_alerts = self.warning_alerts

        info_alerts = self.info_alerts

        total_alerts = self.total_alerts

        first_check: None | str
        if isinstance(self.first_check, datetime.datetime):
            first_check = self.first_check.isoformat()
        else:
            first_check = self.first_check

        last_check: None | str
        if isinstance(self.last_check, datetime.datetime):
            last_check = self.last_check.isoformat()
        else:
            last_check = self.last_check

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "contract_id": contract_id,
                "month": month,
                "uptime_pct": uptime_pct,
                "total_checks": total_checks,
                "healthy_checks": healthy_checks,
                "incidents": incidents,
                "mttr_seconds": mttr_seconds,
                "total_downtime_seconds": total_downtime_seconds,
                "longest_outage_seconds": longest_outage_seconds,
                "ongoing_outage": ongoing_outage,
                "critical_alerts": critical_alerts,
                "warning_alerts": warning_alerts,
                "info_alerts": info_alerts,
                "total_alerts": total_alerts,
                "first_check": first_check,
                "last_check": last_check,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        d = dict(src_dict)
        contract_id = d.pop("contract_id")

        month = d.pop("month")

        uptime_pct = d.pop("uptime_pct")

        total_checks = d.pop("total_checks")

        healthy_checks = d.pop("healthy_checks")

        incidents = d.pop("incidents")

        mttr_seconds = d.pop("mttr_seconds")

        total_downtime_seconds = d.pop("total_downtime_seconds")

        longest_outage_seconds = d.pop("longest_outage_seconds")

        ongoing_outage = d.pop("ongoing_outage")

        critical_alerts = d.pop("critical_alerts")

        warning_alerts = d.pop("warning_alerts")

        info_alerts = d.pop("info_alerts")

        total_alerts = d.pop("total_alerts")

        def _parse_first_check(data: object) -> datetime.datetime | None:
            if data is None:
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                first_check_type_0 = datetime.datetime.fromisoformat(data)

                return first_check_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None, data)

        first_check = _parse_first_check(d.pop("first_check"))

        def _parse_last_check(data: object) -> datetime.datetime | None:
            if data is None:
                return data
            try:
                if not isinstance(data, str):
                    raise TypeError()
                last_check_type_0 = datetime.datetime.fromisoformat(data)

                return last_check_type_0
            except (TypeError, ValueError, AttributeError, KeyError):
                pass
            return cast(datetime.datetime | None, data)

        last_check = _parse_last_check(d.pop("last_check"))

        monthly_sla = cls(
            contract_id=contract_id,
            month=month,
            uptime_pct=uptime_pct,
            total_checks=total_checks,
            healthy_checks=healthy_checks,
            incidents=incidents,
            mttr_seconds=mttr_seconds,
            total_downtime_seconds=total_downtime_seconds,
            longest_outage_seconds=longest_outage_seconds,
            ongoing_outage=ongoing_outage,
            critical_alerts=critical_alerts,
            warning_alerts=warning_alerts,
            info_alerts=info_alerts,
            total_alerts=total_alerts,
            first_check=first_check,
            last_check=last_check,
        )

        monthly_sla.additional_properties = d
        return monthly_sla

    @property
    def additional_keys(self) -> list[str]:
        return list(self.additional_properties.keys())

    def __getitem__(self, key: str) -> Any:
        return self.additional_properties[key]

    def __setitem__(self, key: str, value: Any) -> None:
        self.additional_properties[key] = value

    def __delitem__(self, key: str) -> None:
        del self.additional_properties[key]

    def __contains__(self, key: str) -> bool:
        return key in self.additional_properties
