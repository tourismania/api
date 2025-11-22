<?php

declare(strict_types=1);
declare(ticks=1000);

namespace App\Presentation\Http\Api\V1\GetMe;

use App\Infrastructure\Persistence\Doctrine\User\User;
use Symfony\Bundle\SecurityBundle\Security;
use Symfony\Component\HttpFoundation\Request;
use Symfony\Component\HttpKernel\Controller\ValueResolverInterface;
use Symfony\Component\HttpKernel\ControllerMetadata\ArgumentMetadata;

readonly class GetMeResolver implements ValueResolverInterface
{
    public function __construct(
        private Security $security
    ){}

    public function resolve(Request $request, ArgumentMetadata $argument): iterable
    {
        /** @var User $user */
        $user = $this->security->getUser();

        return [
            new GetMeDto(
                $user->getId(),
                $user->getUserIdentifier(),
                $user->getPhone(),
                $user->getFirstName(),
                $user->getLastName()
            )
        ];
    }
}
