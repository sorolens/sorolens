from __future__ import annotations

from collections.abc import Mapping
from typing import Any, TypeVar, BinaryIO, TextIO, TYPE_CHECKING, Generator

from attrs import define as _attrs_define
from attrs import field as _attrs_field

from ..types import UNSET, Unset







T = TypeVar("T", bound="WatchdogStats")



@_attrs_define
class WatchdogStats:
    """ 
        Attributes:
            total_monitored (int):
            healthy (int):
            degraded (int):
            unresponsive (int):
            total_alerts (int):
            critical_alerts (int):
     """

    total_monitored: int
    healthy: int
    degraded: int
    unresponsive: int
    total_alerts: int
    critical_alerts: int
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)





    def to_dict(self) -> dict[str, Any]:
        total_monitored = self.total_monitored

        healthy = self.healthy

        degraded = self.degraded

        unresponsive = self.unresponsive

        total_alerts = self.total_alerts

        critical_alerts = self.critical_alerts


        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update({
            "total_monitored": total_monitored,
            "healthy": healthy,
            "degraded": degraded,
            "unresponsive": unresponsive,
            "total_alerts": total_alerts,
            "critical_alerts": critical_alerts,
        })

        return field_dict



    @classmethod
    def from_dict(cls: type[T], src_dict: Mapping[str, Any]) -> T:
        d = dict(src_dict)
        total_monitored = d.pop("total_monitored")

        healthy = d.pop("healthy")

        degraded = d.pop("degraded")

        unresponsive = d.pop("unresponsive")

        total_alerts = d.pop("total_alerts")

        critical_alerts = d.pop("critical_alerts")

        watchdog_stats = cls(
            total_monitored=total_monitored,
            healthy=healthy,
            degraded=degraded,
            unresponsive=unresponsive,
            total_alerts=total_alerts,
            critical_alerts=critical_alerts,
        )


        watchdog_stats.additional_properties = d
        return watchdog_stats

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
