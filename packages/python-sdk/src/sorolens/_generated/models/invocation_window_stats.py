from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.invocation_window_stats_failure_categories import (
        InvocationWindowStatsFailureCategories,
    )


T = TypeVar("T", bound="InvocationWindowStats")


@_attrs_define
class InvocationWindowStats:
    """Fees are measured in stroops, CPU in instructions, and failure_rate is a ratio from 0 to 1.

    Attributes:
        invocation_count (int):
        failed_invocations (int):
        failure_rate (float):
        avg_fee (float):
        p95_fee (float):
        avg_cpu (float):
        p95_cpu (float):
        failure_categories (InvocationWindowStatsFailureCategories): Count of the window's failed invocations per
            category. Rows indexed before failure classification existed are counted as `other`, so the counts add up to
            failed_invocations.
    """

    invocation_count: int
    failed_invocations: int
    failure_rate: float
    avg_fee: float
    p95_fee: float
    avg_cpu: float
    p95_cpu: float
    failure_categories: InvocationWindowStatsFailureCategories
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        invocation_count = self.invocation_count

        failed_invocations = self.failed_invocations

        failure_rate = self.failure_rate

        avg_fee = self.avg_fee

        p95_fee = self.p95_fee

        avg_cpu = self.avg_cpu

        p95_cpu = self.p95_cpu

        failure_categories = self.failure_categories.to_dict()

        field_dict: dict[str, Any] = {}
        field_dict.update(self.additional_properties)
        field_dict.update(
            {
                "invocation_count": invocation_count,
                "failed_invocations": failed_invocations,
                "failure_rate": failure_rate,
                "avg_fee": avg_fee,
                "p95_fee": p95_fee,
                "avg_cpu": avg_cpu,
                "p95_cpu": p95_cpu,
                "failure_categories": failure_categories,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.invocation_window_stats_failure_categories import (
            InvocationWindowStatsFailureCategories,
        )

        d = dict(src_dict)
        invocation_count = d.pop("invocation_count")

        failed_invocations = d.pop("failed_invocations")

        failure_rate = d.pop("failure_rate")

        avg_fee = d.pop("avg_fee")

        p95_fee = d.pop("p95_fee")

        avg_cpu = d.pop("avg_cpu")

        p95_cpu = d.pop("p95_cpu")

        failure_categories = InvocationWindowStatsFailureCategories.from_dict(
            d.pop("failure_categories")
        )

        invocation_window_stats = cls(
            invocation_count=invocation_count,
            failed_invocations=failed_invocations,
            failure_rate=failure_rate,
            avg_fee=avg_fee,
            p95_fee=p95_fee,
            avg_cpu=avg_cpu,
            p95_cpu=p95_cpu,
            failure_categories=failure_categories,
        )

        invocation_window_stats.additional_properties = d
        return invocation_window_stats

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
