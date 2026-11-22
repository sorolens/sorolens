from __future__ import annotations

from collections.abc import Mapping
from typing import TYPE_CHECKING, Any, Self, TypeVar

from attrs import define as _attrs_define
from attrs import field as _attrs_field
from typing_extensions import Self

if TYPE_CHECKING:
    from ..models.failure_counts_by_category import FailureCountsByCategory


T = TypeVar("T", bound="InvocationWindowStats")


@_attrs_define
class InvocationWindowStats:
    """Fees are measured in stroops, CPU in instructions, and failure_rate is a ratio from 0 to 1. failures_by_category
    counts classified failures; failed invocations indexed before classification existed count toward failed_invocations
    only, so the categories can sum to less.

        Attributes:
            invocation_count (int):
            failed_invocations (int):
            failure_rate (float):
            avg_fee (float):
            p95_fee (float):
            avg_cpu (float):
            p95_cpu (float):
            failures_by_category (FailureCountsByCategory): Classified failures per FailureCategory. Every category is
                present, with 0 when it has none.
    """

    invocation_count: int
    failed_invocations: int
    failure_rate: float
    avg_fee: float
    p95_fee: float
    avg_cpu: float
    p95_cpu: float
    failures_by_category: FailureCountsByCategory
    additional_properties: dict[str, Any] = _attrs_field(init=False, factory=dict)

    def to_dict(self) -> dict[str, Any]:
        invocation_count = self.invocation_count

        failed_invocations = self.failed_invocations

        failure_rate = self.failure_rate

        avg_fee = self.avg_fee

        p95_fee = self.p95_fee

        avg_cpu = self.avg_cpu

        p95_cpu = self.p95_cpu

        failures_by_category = self.failures_by_category.to_dict()

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
                "failures_by_category": failures_by_category,
            }
        )

        return field_dict

    @classmethod
    def from_dict(cls, src_dict: Mapping[str, Any]) -> Self:
        from ..models.failure_counts_by_category import FailureCountsByCategory

        d = dict(src_dict)
        invocation_count = d.pop("invocation_count")

        failed_invocations = d.pop("failed_invocations")

        failure_rate = d.pop("failure_rate")

        avg_fee = d.pop("avg_fee")

        p95_fee = d.pop("p95_fee")

        avg_cpu = d.pop("avg_cpu")

        p95_cpu = d.pop("p95_cpu")

        failures_by_category = FailureCountsByCategory.from_dict(
            d.pop("failures_by_category")
        )

        invocation_window_stats = cls(
            invocation_count=invocation_count,
            failed_invocations=failed_invocations,
            failure_rate=failure_rate,
            avg_fee=avg_fee,
            p95_fee=p95_fee,
            avg_cpu=avg_cpu,
            p95_cpu=p95_cpu,
            failures_by_category=failures_by_category,
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
