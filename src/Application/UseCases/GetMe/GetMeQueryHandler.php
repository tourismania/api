<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Application\UseCases\GetMe;

use App\Domain\Services\RightsDescriber;
use Symfony\Component\Messenger\Attribute\AsMessageHandler;

#[AsMessageHandler(bus: 'query.bus')]
readonly class GetMeQueryHandler
{
    public function __construct(
        private RightsDescriber $rightsDescriber,
    ) {
    }

    public function __invoke(GetMeQuery $getMeQuery): GetMeResult
    {
        return new GetMeResult(
            $getMeQuery->id,
            $getMeQuery->email,
            $getMeQuery->phone,
            $getMeQuery->firstName,
            $getMeQuery->lastName,
            $this->rightsDescriber->byRoles($getMeQuery->roles),
        );
    }
}
